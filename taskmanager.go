package main

import (
	"context"
	"sync"

	"github.com/aidarkhanov/nanoid/v2"
)

type TaskManager struct {
	// mu    sync.Mutex
	tasks     map[string]*Task
	Context   context.Context
	WaitGroup sync.WaitGroup
}

type Task struct {
	// mu      sync.Mutex
	id        string
	ctx       context.Context
	cancelCtx context.CancelFunc
	handler   func(ctx context.Context, wg *sync.WaitGroup, rest ...interface{})
	args      []interface{}
}

type TaskId struct{}

func NewTaskManager() *TaskManager {
	tm := new(TaskManager)
	tm.Context = context.Background()
	tm.tasks = make(map[string]*Task)
	return tm
}

func (tm *TaskManager) AddTask(handler func(ctx context.Context, wg *sync.WaitGroup, rest ...interface{}), args ...interface{}) *Task {
	id, _ := nanoid.New()
	ctx, cancelCtx := context.WithCancel(tm.Context)
	ctx = context.WithValue(ctx, TaskId{}, id)

	tm.tasks[id] = &Task{id: id, handler: handler, ctx: ctx, cancelCtx: cancelCtx, args: args}
	return tm.tasks[id]
}

func (tm *TaskManager) StartTask(id string) *Task {
	task := tm.tasks[id]

	tm.WaitGroup.Add(1)
	go task.handler(task.ctx, &tm.WaitGroup, task.args...)
	return task
}

func (tm *TaskManager) StopTask(id string) *Task {
	task := tm.tasks[id]

	task.cancelCtx()

	return task
}

func (tm *TaskManager) StartTasks() {
	for taskId := range tm.tasks {
		tm.StartTask(taskId)
	}
}

func (tm *TaskManager) StopTasks() {
	for taskId := range tm.tasks {
		tm.StopTask(taskId)
	}
}
