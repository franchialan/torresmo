package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/torresmo-dev/torresmo/internal/storage"
)

var eventsCmd = &cobra.Command{
	Use:   "events [resource/name]",
	Short: "View cluster events",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		clusterFlag, _ := cmd.Flags().GetString("cluster")
		clusterName, err := resolveCluster(clusterFlag)
		if err != nil {
			return err
		}

		namespace, _ := cmd.Flags().GetString("namespace")
		watch, _ := cmd.Flags().GetBool("watch")
		typeFilter, _ := cmd.Flags().GetString("type")

		o, err := getOrchestrator()
		if err != nil {
			return err
		}

		fetchEvents := func() ([]storage.EventRecord, error) {
			if len(args) == 1 {
				parts := strings.SplitN(args[0], "/", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("resource must be in kind/name format (e.g., pod/my-api)")
				}
				return o.ListEventsByResource(ctx, clusterName, parts[0], parts[1], 50)
			}
			return o.ListEvents(ctx, clusterName, namespace, 50)
		}

		if !watch {
			events, err := fetchEvents()
			if err != nil {
				return fmt.Errorf("failed to list events: %w", err)
			}
			printEvents(events, typeFilter)
			return nil
		}

		// Watch mode: poll every 2 seconds, track last seen event ID.
		var lastID int64
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		// Print initial set.
		events, err := fetchEvents()
		if err != nil {
			return fmt.Errorf("failed to list events: %w", err)
		}
		printEvents(events, typeFilter)
		if len(events) > 0 {
			lastID = events[len(events)-1].ID
		}

		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				events, err := fetchEvents()
				if err != nil {
					fmt.Fprintf(os.Stderr, "error fetching events: %v\n", err)
					continue
				}
				for _, e := range events {
					if e.ID > lastID {
						if typeFilter != "" && !strings.EqualFold(e.Type, typeFilter) {
							continue
						}
						ts := time.Unix(e.CreatedAt, 0).Format("15:04:05")
						fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n",
							ts, e.Type, e.Kind, e.Name, e.Reason, e.Message)
						lastID = e.ID
					}
				}
			}
		}
	},
}

func printEvents(events []storage.EventRecord, typeFilter string) {
	if len(events) == 0 {
		fmt.Println("No events found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "TIMESTAMP\tTYPE\tKIND\tNAME\tREASON\tMESSAGE")
	for _, e := range events {
		if typeFilter != "" && !strings.EqualFold(e.Type, typeFilter) {
			continue
		}
		ts := time.Unix(e.CreatedAt, 0).Format("15:04:05")
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			ts, e.Type, e.Kind, e.Name, e.Reason, e.Message)
	}
	_ = w.Flush()
}

func init() {
	eventsCmd.Flags().String("cluster", "", "Cluster name")
	eventsCmd.Flags().StringP("namespace", "n", "", "Namespace")
	eventsCmd.Flags().Bool("watch", false, "Watch for new events")
	eventsCmd.Flags().String("type", "", "Filter by event type (Normal, Warning)")
	addCommand(eventsCmd)
}
