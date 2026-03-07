package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/alamo-ds/msgraph/graph"
)

var (
	errTypeCast = errors.New("casting from in channel failed")
	workerErr   = func(worker string, err error) error {
		return fmt.Errorf("worker type %q encountered an error:\n%v", worker, err)
	}
)

type client struct {
	c          *graph.Client
	rootWorker op
	dag        *DAG
}

func newClient(c *graph.Client) (*client, error) {
	client := &client{
		c: c,
	}
	client.rootWorker = func(ctx context.Context, in <-chan any, out chan<- any) {
		rbGroups := client.c.Groups()
		groups, err := rbGroups.Get(ctx)
		if err != nil {
			client.Error("root", err)
			return
		}

		for _, group := range groups {
			out <- groupJob{
				id:        group.ID,
				rbGroup:   rbGroups.ById(group.ID),
				rbPlanner: client.c.Planner(),
			}
		}
	}

	dag, err := NewDAG([]*node{
		newNode("rootIn", client.rootWorker),
		newNode("groups worker", client.groupWorker, "rootIn"),
		newNode("plans worker", client.planWorker, "groups worker"),
		newNode("tasks worker", client.taskWorker, "plans worker"),
	})
	if err != nil {
		return nil, err
	}

	client.dag = dag

	return client, nil
}

// TODO: inject context here
func (cl *client) execute(ctx context.Context) <-chan any {
	return cl.dag.Run(ctx)
}

func (cl *client) Error(worker string, err error) {
	cl.dag.errCh <- workerErr(worker, err)
}

func (cl *client) Close() {
	slog.Info("closing client...")
}

type groupJob struct {
	id        string
	rbGroup   *graph.GroupItemRequestBuilder
	rbPlanner *graph.PlannerRequestBuilder
}

func (cl *client) groupWorker(ctx context.Context, in <-chan any, out chan<- any) {
	for job := range in {
		groupJob, ok := job.(groupJob)
		if !ok {
			cl.Error("group", errTypeCast)
			return
		}

		plans, err := groupJob.rbGroup.Plans().Get(ctx)
		if err != nil {
			cl.Error("group", err)
			return
		}

		for _, plan := range plans {
			out <- planJob{
				rbPlannerTasks: groupJob.rbPlanner.ById(plan.ID).Tasks(),
				rbThreads:      groupJob.rbGroup.Threads(),
				rbTasks:        groupJob.rbPlanner.Tasks(),
			}
		}
	}
}

type planJob struct {
	rbPlannerTasks *graph.TasksRequestBuilder
	rbThreads      *graph.ThreadsRequestBuilder
	rbTasks        *graph.TasksRequestBuilder
}

func (cl *client) planWorker(ctx context.Context, in <-chan any, out chan<- any) {
	for job := range in {
		planJob, ok := job.(planJob)
		if !ok {
			cl.Error("plan", errTypeCast)
			return
		}

		tasks, err := planJob.rbPlannerTasks.Get(ctx)
		if err != nil {
			cl.Error("plan", err)
			continue
		}

		for _, task := range tasks {
			taskJob := taskJob{
				task:   task,
				rbTask: planJob.rbTasks.ById(task.ID),
			}
			if task.ConversationThreadID != "" {
				taskJob.rbPosts = planJob.rbThreads.ById(task.ConversationThreadID)
			}
			out <- taskJob
		}
	}
}

type taskJob struct {
	task    graph.Task
	rbTask  *graph.TaskRequestBuilder
	rbPosts *graph.PostsRequestBuilder
}

func (cl *client) taskWorker(ctx context.Context, in <-chan any, out chan<- any) {
	for job := range in {
		taskJob, ok := job.(taskJob)
		if !ok {
			cl.Error("task", errTypeCast)
			return
		}

		details, err := taskJob.rbTask.Details().Get(ctx)
		if err != nil {
			cl.Error("task details", err)
			continue
		}

		var posts []graph.Post
		if taskJob.rbPosts != nil {
			posts, err = taskJob.rbPosts.Get(ctx)
			if err != nil {
				cl.Error("task comments", err)
				continue
			}
		}

		out <- NewTaskFromGraph(taskJob.task).
			AddDetails(details).
			AddComments(posts)
	}

}
