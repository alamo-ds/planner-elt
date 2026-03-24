package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/alamo-ds/dag"
	"github.com/alamo-ds/msgraph/graph"
)

var (
	errTypeCast = errors.New("casting from in channel failed")
	workerErr   = func(worker string, err error) error {
		return fmt.Errorf("worker type %q encountered an error:\n%v", worker, err)
	}
)

type client struct {
	c *graph.Client
	d *dag.DAG
}

func newClient(c *graph.Client) (*client, error) {
	client := &client{
		c: c,
	}

	var rootWorker = func(ctx context.Context, in <-chan any, out chan<- any) error {
		rbGroups := client.c.Groups()
		groups, err := rbGroups.Get(ctx)
		if err != nil {
			return workerErr("root", err)
		}

		for _, group := range groups {
			out <- groupJob{
				id:        group.ID,
				rbGroup:   rbGroups.ById(group.ID),
				rbPlanner: client.c.Planner(),
			}
		}

		return nil
	}

	d, err := dag.NewDag(
		dag.Node("rootIn", rootWorker),
		dag.Node("groups worker", groupWorker, "rootIn"),
		dag.Node("plans worker", planWorker, "groups worker"),
		dag.Node("tasks worker", client.taskWorker, "plans worker"),
	)
	if err != nil {
		return nil, err
	}

	client.d = d

	return client, nil
}

// TODO: inject context here
func (cl *client) execute(ctx context.Context) <-chan any {
	return cl.d.Run(ctx)
}

func (cl *client) Close() {
	slog.Info("closing client...")
}

type groupJob struct {
	id        string
	rbGroup   *graph.GroupItemRequestBuilder
	rbPlanner *graph.PlannerRequestBuilder
}

func groupWorker(ctx context.Context, in <-chan any, out chan<- any) error {
	for job := range in {
		groupJob, ok := job.(groupJob)
		if !ok {
			return errTypeCast
		}

		plans, err := groupJob.rbGroup.Plans().Get(ctx)
		if err != nil {
			return workerErr("group", err)
		}

		for _, plan := range plans {
			out <- planJob{
				rbPlannerTasks: groupJob.rbPlanner.ById(plan.ID).Tasks(),
				rbThreads:      groupJob.rbGroup.Threads(),
				rbTasks:        groupJob.rbPlanner.Tasks(),
			}
		}
	}

	return nil
}

type planJob struct {
	rbPlannerTasks *graph.TasksRequestBuilder
	rbThreads      *graph.ThreadsRequestBuilder
	rbTasks        *graph.TasksRequestBuilder
}

func planWorker(ctx context.Context, in <-chan any, out chan<- any) error {
	for job := range in {
		planJob, ok := job.(planJob)
		if !ok {
			return errTypeCast
		}

		tasks, err := planJob.rbPlannerTasks.Get(ctx)
		if err != nil {
			return workerErr("plan", err)
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

	return nil
}

type taskJob struct {
	task    graph.Task
	rbTask  *graph.TaskRequestBuilder
	rbPosts *graph.PostsRequestBuilder
}

func (cl *client) taskWorker(ctx context.Context, in <-chan any, out chan<- any) error {
	userMap := make(users)
	users, err := cl.c.Users().Get(ctx)
	if err != nil {
		return workerErr("task", err)
	}

	for _, user := range users {
		userMap.add(user)
	}

	for job := range in {
		taskJob, ok := job.(taskJob)
		if !ok {
			return errTypeCast
		}

		details, err := taskJob.rbTask.Details().Get(ctx)
		if err != nil {
			return workerErr("task", err)
		}

		var posts []graph.Post
		if taskJob.rbPosts != nil {
			posts, err = taskJob.rbPosts.Get(ctx)
			if err != nil {
				return workerErr("task", err)
			}
		}

		out <- NewTaskFromGraph(taskJob.task).
			AddDetails(details).
			AddComments(posts).
			AddUsers(userMap)
	}

	return nil
}
