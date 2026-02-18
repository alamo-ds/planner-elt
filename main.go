package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/alamo-ds/msgraph/graph"
)

var (
	tenantId     = os.Getenv("tenantid")
	clientId     = os.Getenv("clientid")
	clientSecret = os.Getenv("clientsecret")
)

var cfg = graph.AzureADConfig{
	TenantID:     tenantId,
	ClientID:     clientId,
	ClientSecret: clientSecret,
}

func main() {
	if err := checkCfg(); err != nil {
		log.Fatalln(err)
	}

	blobClient, err := newBlobClient()
	if err != nil {
		log.Fatalln("couldn't create blob client:", err)
	}
	log.Println("blob client created...")

	ctx := context.Background()
	client, err := newClient(graph.NewClient(ctx, cfg))
	if err != nil {
		log.Fatalln("couldn't create MS graph client:", err)
	}
	log.Println("MS graph clinet created...")
	defer client.Close()

	log.Println("starting root worker...")

	var tasks []Task

	for r := range client.execute(context.Background()) {
		task, ok := r.(*Task)
		if !ok {
			log.Fatalln("type mismatch!")
		}
		tasks = append(tasks, *task)
	}

	if len(tasks) == 0 {
		log.Fatalln("GetTaskJob did not return any results")
	}

	fmt.Println("tasks found:", len(tasks))

	if err = pushBlob(ctx, blobClient, tasksDir, tasks); err != nil {
		log.Fatalln("couldn't push tasks to ADLS:", err)
	}

	log.Println("program ran successfully. exiting...")
}

func checkCfg() error {
	m := []string{}

	if cfg.TenantID == "" {
		m = append(m, "tenantid")
	}
	if cfg.ClientID == "" {
		m = append(m, "clientid")
	}
	if cfg.ClientSecret == "" {
		m = append(m, "clientsecret")
	}

	if len(m) == 0 {
		return nil
	}

	var sb strings.Builder
	for i, envVar := range m {
		sb.WriteString("  " + envVar)
		if i < len(m)-1 {
			sb.WriteString("\n")
		}
	}

	return errors.New("please set the following variables:\n" + sb.String())
}
