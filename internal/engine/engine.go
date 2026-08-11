package engine

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gatra-io/gatra/internal/config"
	"github.com/gatra-io/gatra/internal/errors"
	"github.com/gatra-io/gatra/internal/evaluator"
	"github.com/gatra-io/gatra/internal/storage"
)

type RuleState struct {
	Sum         float64
	Count       int64
	UniqueSet   map[string]struct{}
	WindowStart time.Time
}

type TrajectoryState struct {
	LastAccess time.Time
	Rules      map[string]*RuleState
}

type Engine struct {
	mu           sync.RWMutex
	trajectories map[string]*TrajectoryState
	ttl          time.Duration
	logger       *slog.Logger
	store        *storage.Store
}

// NewEngine constructs an initialized state engine, loading persisted session records from disk.
func NewEngine(ttl time.Duration, log *slog.Logger, store *storage.Store) (*Engine, error) {
	e := &Engine{
		trajectories: make(map[string]*TrajectoryState),
		ttl:          ttl,
		logger:       log,
		store:        store,
	}

	if store != nil {
		loaded, err := store.LoadAllTrajectories()
		if err != nil {
			return nil, fmt.Errorf("failed to restore trajectories from storage: %w", err)
		}

		for id, data := range loaded {
			traj := &TrajectoryState{
				LastAccess: data.LastAccess,
				Rules:      make(map[string]*RuleState),
			}
			for rID, rData := range data.Rules {
				traj.Rules[rID] = &RuleState{
					Sum:         rData.Sum,
					Count:       rData.Count,
					UniqueSet:   rData.UniqueSet,
					WindowStart: rData.WindowStart,
				}
			}
			e.trajectories[id] = traj
		}
		e.logger.Info("restored persisted session trajectories from disk", slog.Int("count", len(loaded)))
	}

	if ttl > 0 {
		go e.startJanitor(1 * time.Minute)
	}

	return e, nil
}

func (e *Engine) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		e.mu.Lock()
		now := time.Now()
		purged := 0
		for id, traj := range e.trajectories {
			if now.Sub(traj.LastAccess) > e.ttl {
				delete(e.trajectories, id)
				if e.store != nil {
					_ = e.store.DeleteTrajectory(id)
				}
				purged++
			}
		}
		if purged > 0 {
			e.logger.Info("background janitor purged stale sessions from RAM and disk", slog.Int("count", purged))
		}
		e.mu.Unlock()
	}
}

func isWindowExpired(tw *config.TimeWindow, windowStart time.Time, now time.Time) bool {
	if tw == nil || !tw.Enabled {
		return false
	}

	loc := time.UTC
	if tw.Timezone != "" {
		if loadedLoc, err := time.LoadLocation(tw.Timezone); err == nil {
			loc = loadedLoc
		}
	}

	nowInLoc := now.In(loc)
	startInLoc := windowStart.In(loc)

	switch tw.ResetSchedule {
	case "hourly":
		return nowInLoc.Hour() != startInLoc.Hour() || nowInLoc.YearDay() != startInLoc.YearDay()
	case "daily":
		return nowInLoc.YearDay() != startInLoc.YearDay() || nowInLoc.Year() != startInLoc.Year()
	default:
		return false
	}
}

func (e *Engine) EvaluateAndIncrement(trajectoryID string, rule config.Rule, evalRes *evaluator.EvaluationResult) error {
	if rule.Limits.MaxCumulative == nil {
		return nil
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	traj, exists := e.trajectories[trajectoryID]
	if !exists {
		traj = &TrajectoryState{
			Rules: make(map[string]*RuleState),
		}
		e.trajectories[trajectoryID] = traj
	}
	traj.LastAccess = now

	state, exists := traj.Rules[rule.RuleID]
	if !exists {
		state = &RuleState{
			UniqueSet:   make(map[string]struct{}),
			WindowStart: now,
		}
		traj.Rules[rule.RuleID] = state
	}

	if isWindowExpired(rule.TimeWindow, state.WindowStart, now) {
		e.logger.Info("⏰ time-window schedule reset triggered",
			slog.String("rule_id", rule.RuleID),
			slog.String("trajectory_id", trajectoryID),
			slog.String("schedule", rule.TimeWindow.ResetSchedule),
		)
		state.Sum = 0
		state.Count = 0
		state.UniqueSet = make(map[string]struct{})
		state.WindowStart = now
	}

	limit := *rule.Limits.MaxCumulative

	switch rule.Accumulator.Type {
	case config.AccumulatorSum:
		if evalRes.NumericValue == nil {
			return nil
		}
		requestedVal := *evalRes.NumericValue
		if state.Sum+requestedVal > limit {
			return errors.NewProxyError(
				403,
				rule.RuleID,
				fmt.Errorf("%w: current sum %.2f + requested %.2f exceeds cumulative threshold %.2f",
					errors.ErrCumulativeLimitExceeded, state.Sum, requestedVal, limit),
			)
		}
		state.Sum += requestedVal

	case config.AccumulatorCount:
		if float64(state.Count+1) > limit {
			return errors.NewProxyError(
				403,
				rule.RuleID,
				fmt.Errorf("%w: invocation count %d exceeds maximum allowed %f",
					errors.ErrCumulativeLimitExceeded, state.Count+1, limit),
			)
		}
		state.Count++

	case config.AccumulatorUniqueSet:
		var target string
		if evalRes.StringValue != nil {
			target = *evalRes.StringValue
		} else {
			return nil
		}

		_, alreadyPresent := state.UniqueSet[target]
		currentSize := len(state.UniqueSet)

		if !alreadyPresent && float64(currentSize+1) > limit {
			return errors.NewProxyError(
				403,
				rule.RuleID,
				fmt.Errorf("%w: target '%s' increases unique set size to %d, exceeding limit %f",
					errors.ErrUniqueSetLimitExceeded, target, currentSize+1, limit),
			)
		}

		state.UniqueSet[target] = struct{}{}
	}

	// Persist state update to bbolt disk database
	if e.store != nil {
		rulesMap := make(map[string]storage.RuleStateData)
		for rID, rState := range traj.Rules {
			rulesMap[rID] = storage.RuleStateData{
				Sum:         rState.Sum,
				Count:       rState.Count,
				UniqueSet:   rState.UniqueSet,
				WindowStart: rState.WindowStart,
			}
		}
		_ = e.store.SaveTrajectory(trajectoryID, storage.TrajectoryData{
			LastAccess: traj.LastAccess,
			Rules:      rulesMap,
		})
	}

	return nil
}