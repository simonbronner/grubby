package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"grubby/internal/github"
)

type TaskUpdateAction string

const (
	ActionUpdateTask   TaskUpdateAction = "update"
	ActionCloseTask    TaskUpdateAction = "close"
	ActionReopenTask   TaskUpdateAction = "reopen"
	ActionPriorityTask TaskUpdateAction = "priority"
	ActionAssignTask   TaskUpdateAction = "assign"
)

type TaskDebounceJob struct {
	Key            string
	TaskID         int64
	Number         int
	RepoOwner      string
	RepoName       string
	Title          string
	Body           string
	Priority       github.TaskPriority
	Assignees      []string
	Labels         []string
	IsSquad        bool
	Action         TaskUpdateAction
	EnqueuedAt     time.Time
	CoalescedCount int
	timer          *time.Timer
	flushFn        func(ctx context.Context, job *TaskDebounceJob) error
}

type TaskDebouncer struct {
	mu              sync.Mutex
	window          time.Duration
	pending         map[string]*TaskDebounceJob
	onFlushComplete func(job *TaskDebounceJob, err error)
	onCoalesced     func()
}

func NewTaskDebouncer(window time.Duration, onCoalesced func(), onFlushComplete func(job *TaskDebounceJob, err error)) *TaskDebouncer {
	if window <= 0 {
		window = 1500 * time.Millisecond // 1.5s default debounce window
	}
	return &TaskDebouncer{
		window:          window,
		pending:         make(map[string]*TaskDebounceJob),
		onFlushComplete: onFlushComplete,
		onCoalesced:     onCoalesced,
	}
}

// JobKey generates a unique key for task debouncing
func JobKey(repoOwner, repoName string, number int, taskID int64) string {
	if number > 0 && repoName != "" {
		return fmt.Sprintf("%s/%s#%d", repoOwner, repoName, number)
	}
	return fmt.Sprintf("task-id-%d", taskID)
}

// Enqueue registers or coalesces a task modification
func (d *TaskDebouncer) Enqueue(job TaskDebounceJob, flushFn func(ctx context.Context, job *TaskDebounceJob) error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := job.Key
	if key == "" {
		key = JobKey(job.RepoOwner, job.RepoName, job.Number, job.TaskID)
		job.Key = key
	}

	if existing, found := d.pending[key]; found {
		// Coalesce into existing job
		if existing.timer != nil {
			existing.timer.Stop()
		}
		existing.CoalescedCount++
		if job.Title != "" {
			existing.Title = job.Title
		}
		if job.Body != "" {
			existing.Body = job.Body
		}
		if job.Priority != "" {
			existing.Priority = job.Priority
		}
		if len(job.Assignees) > 0 {
			existing.Assignees = job.Assignees
		}
		if len(job.Labels) > 0 {
			existing.Labels = job.Labels
		}
		if job.Action != "" {
			existing.Action = job.Action
		}
		if flushFn != nil {
			existing.flushFn = flushFn
		}

		if d.onCoalesced != nil {
			go d.onCoalesced()
		}

		// Reset debounce timer
		existing.timer = time.AfterFunc(d.window, func() {
			d.triggerFlush(key)
		})
		return
	}

	// New job
	job.EnqueuedAt = time.Now()
	job.flushFn = flushFn

	newJob := &job
	newJob.timer = time.AfterFunc(d.window, func() {
		d.triggerFlush(key)
	})

	d.pending[key] = newJob
}

func (d *TaskDebouncer) triggerFlush(key string) {
	d.mu.Lock()
	job, found := d.pending[key]
	if !found {
		d.mu.Unlock()
		return
	}
	delete(d.pending, key)
	d.mu.Unlock()

	if job.flushFn != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		err := job.flushFn(ctx, job)
		if d.onFlushComplete != nil {
			d.onFlushComplete(job, err)
		}
	}
}

// FlushAll executes all pending debounced jobs immediately
func (d *TaskDebouncer) FlushAll(ctx context.Context) {
	d.mu.Lock()
	jobs := make([]*TaskDebounceJob, 0, len(d.pending))
	for k, job := range d.pending {
		if job.timer != nil {
			job.timer.Stop()
		}
		delete(d.pending, k)
		jobs = append(jobs, job)
	}
	d.mu.Unlock()

	for _, job := range jobs {
		if job.flushFn != nil {
			err := job.flushFn(ctx, job)
			if d.onFlushComplete != nil {
				d.onFlushComplete(job, err)
			}
		}
	}
}

// PendingCount returns the number of pending debounced write jobs
func (d *TaskDebouncer) PendingCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.pending)
}
