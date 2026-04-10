package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

const defaultAPIURL = "http://localhost:8080"

var apiURL string

func main() {
	rootCmd := &cobra.Command{
		Use:   "dockclawctl",
		Short: "ZClaw CLI — manage agents, providers, and tasks",
		PersistentPreRun: func(_ *cobra.Command, _ []string) {
			if apiURL == "" {
				apiURL = envOr("ZCLAW_API_URL", defaultAPIURL)
			}
		},
	}
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "API server URL (default: http://localhost:8080)")

	// Agent commands.
	agentCmd := &cobra.Command{Use: "agent", Aliases: []string{"agents"}, Short: "Manage agents"}

	agentCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List all agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			state, _ := cmd.Flags().GetString("state")
			limit, _ := cmd.Flags().GetInt("limit")
			resp, err := apiGet(fmt.Sprintf("/api/v1/agents?limit=%d&state=%s", limit, state))
			if err != nil {
				return err
			}
			var list struct {
				Agents []struct {
					ID        string `json:"id"`
					Name      string `json:"name"`
					State     string `json:"state"`
					Provider  string `json:"provider"`
					Model     string `json:"model"`
					CreatedAt string `json:"created_at"`
				} `json:"agents"`
				Total int `json:"total"`
			}
			json.Unmarshal(resp, &list)

			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintf(tw, "ID\tNAME\tSTATE\tPROVIDER\tMODEL\tCREATED\n")
			for _, a := range list.Agents {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
					a.ID[:8], a.Name, a.State, a.Provider, a.Model, a.CreatedAt[:10])
			}
			tw.Flush()
			fmt.Fprintf(os.Stderr, "Total: %d\n", list.Total)
			return nil
		},
	})
	agentCmd.Commands()[0].Flags().String("state", "", "filter by state")
	agentCmd.Commands()[0].Flags().Int("limit", 50, "max results")

	agentCmd.AddCommand(&cobra.Command{
		Use:   "create [name]",
		Short: "Create a new agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, _ := cmd.Flags().GetString("provider")
			model, _ := cmd.Flags().GetString("model")
			schedule, _ := cmd.Flags().GetString("schedule")
			systemPrompt, _ := cmd.Flags().GetString("prompt")

			body := map[string]any{
				"name": args[0],
				"provider": map[string]any{
					"provider_id": provider,
					"model":       model,
				},
				"schedule": map[string]any{
					"cron":    schedule,
					"enabled": schedule != "",
				},
			}
			if systemPrompt != "" {
				body["provider"].(map[string]any)["system_prompt"] = systemPrompt
			}

			resp, err := apiPost("/api/v1/agents", body)
			if err != nil {
				return err
			}
			var agent map[string]any
			json.Unmarshal(resp, &agent)
			fmt.Printf("Created agent: %s (id: %s)\n", args[0], agent["id"])
			return nil
		},
	})
	agentCmd.Commands()[1].Flags().String("provider", "openai", "provider ID")
	agentCmd.Commands()[1].Flags().String("model", "gpt-4o-mini", "model name")
	agentCmd.Commands()[1].Flags().String("schedule", "", "cron schedule")
	agentCmd.Commands()[1].Flags().String("prompt", "", "system prompt")

	agentCmd.AddCommand(&cobra.Command{
		Use:   "get [id]",
		Short: "Get agent details",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/agents/" + args[0])
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	agentCmd.AddCommand(&cobra.Command{
		Use:   "delete [id]",
		Short: "Delete an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, err := apiDelete("/api/v1/agents/" + args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Deleted agent: %s\n", args[0])
			return nil
		},
	})

	agentCmd.AddCommand(&cobra.Command{
		Use:   "pause [id]",
		Short: "Pause an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return patchAgentState(args[0], "paused")
		},
	})

	agentCmd.AddCommand(&cobra.Command{
		Use:   "resume [id]",
		Short: "Resume a paused agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return patchAgentState(args[0], "active")
		},
	})

	agentCmd.AddCommand(&cobra.Command{
		Use:   "disable [id]",
		Short: "Disable an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return patchAgentState(args[0], "disabled")
		},
	})

	// Task commands.
	taskCmd := &cobra.Command{Use: "task", Aliases: []string{"tasks"}, Short: "Manage tasks"}

	taskCmd.AddCommand(&cobra.Command{
		Use:   "enqueue [agent-id] [input]",
		Short: "Enqueue a task for an agent",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			body := map[string]string{
				"agent_id": args[0],
				"input":    strings.Join(args[1:], " "),
			}
			resp, err := apiPost("/api/v1/tasks", body)
			if err != nil {
				return err
			}
			var task map[string]any
			json.Unmarshal(resp, &task)
			fmt.Printf("Enqueued task: %s\n", task["id"])
			return nil
		},
	})

	// Stats command.
	statsCmd := &cobra.Command{
		Use:   "stats",
		Short: "Show system stats",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/stats")
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	}

	// Provider commands.
	providerCmd := &cobra.Command{Use: "provider", Aliases: []string{"providers"}, Short: "List providers"}

	providerCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered providers",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/providers")
			if err != nil {
				return err
			}
			var ids []string
			json.Unmarshal(resp, &ids)
			for _, id := range ids {
				fmt.Println(id)
			}
			return nil
		},
	})

	// Doctor command (local check).
	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check connectivity to ZClaw API",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := http.Get(apiURL + "/health")
			if err != nil {
				fmt.Printf("[FAIL] Cannot reach API at %s: %v\n", apiURL, err)
				os.Exit(1)
			}
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Printf("[OK] API healthy at %s\n", apiURL)
			} else {
				fmt.Printf("[WARN] API returned status %d\n", resp.StatusCode)
			}
			return nil
		},
	}

	// Tool commands.
	toolCmd := &cobra.Command{Use: "tool", Aliases: []string{"tools"}, Short: "List and execute tools"}

	toolCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered tools",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/tools")
			if err != nil {
				return err
			}
			var tools []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Category    string `json:"category"`
			}
			json.Unmarshal(resp, &tools)

			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintf(tw, "ID\tNAME\tCATEGORY\tDESCRIPTION\n")
			for _, t := range tools {
				desc := t.Description
				if len(desc) > 50 {
					desc = desc[:50] + "..."
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", t.ID, t.Name, t.Category, desc)
			}
			tw.Flush()
			return nil
		},
	})

	toolCmd.AddCommand(&cobra.Command{
		Use:   "get [id]",
		Short: "Get tool details",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/tools/" + args[0])
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	toolCmd.AddCommand(&cobra.Command{
		Use:   "execute [id] [json-params]",
		Short: "Execute a tool with JSON params",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			params := map[string]any{}
			if len(args) > 1 {
				json.Unmarshal([]byte(strings.Join(args[1:], " ")), &params)
			}
			resp, err := apiPost("/api/v1/tools/"+args[0]+"/execute", params)
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	// Connection commands.
	connCmd := &cobra.Command{Use: "connection", Aliases: []string{"connections"}, Short: "View connection status"}

	connCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List connection statuses",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/connections")
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	// Sub-agent commands.
	subagentCmd := &cobra.Command{Use: "subagent", Aliases: []string{"subagents"}, Short: "Manage sub-agents"}

	subagentCmd.AddCommand(&cobra.Command{
		Use:   "spawn [parent-id] [name] [task]",
		Short: "Spawn a new sub-agent",
		Args:  cobra.MinimumNArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			body := map[string]any{
				"parent_id":        args[0],
				"name":             args[1],
				"task_description": strings.Join(args[2:], " "),
			}
			resp, err := apiPost("/api/v1/subagents", body)
			if err != nil {
				return err
			}
			var sa map[string]any
			json.Unmarshal(resp, &sa)
			fmt.Printf("Spawned sub-agent: %s (id: %s)\n", args[1], sa["id"])
			return nil
		},
	})

	subagentCmd.AddCommand(&cobra.Command{
		Use:   "list [parent-id]",
		Short: "List sub-agents for a parent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			resp, err := apiGet("/api/v1/subagents/parent/" + args[0])
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	subagentCmd.AddCommand(&cobra.Command{
		Use:   "cancel [id]",
		Short: "Cancel a sub-agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, err := apiPost("/api/v1/subagents/"+args[0]+"/cancel", nil)
			if err != nil {
				return err
			}
			fmt.Printf("Cancelled sub-agent: %s\n", args[0])
			return nil
		},
	})

	// Template commands.
	templateCmd := &cobra.Command{Use: "template", Aliases: []string{"templates"}, Short: "Manage agent templates"}

	templateCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List available templates",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/templates")
			if err != nil {
				return err
			}
			var templates []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			json.Unmarshal(resp, &templates)

			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintf(tw, "NAME\tDESCRIPTION\n")
			for _, t := range templates {
				fmt.Fprintf(tw, "%s\t%s\n", t.Name, t.Description)
			}
			tw.Flush()
			return nil
		},
	})

	templateCmd.AddCommand(&cobra.Command{
		Use:   "instantiate [name] [parent-id]",
		Short: "Create an agent from a template",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			body := map[string]string{"parent_id": args[1]}
			resp, err := apiPost("/api/v1/templates/"+args[0]+"/instantiate", body)
			if err != nil {
				return err
			}
			var agent map[string]any
			json.Unmarshal(resp, &agent)
			fmt.Printf("Instantiated agent from template %s: %s\n", args[0], agent["id"])
			return nil
		},
	})

	// Dashboard commands.
	dashboardCmd := &cobra.Command{Use: "dashboard", Aliases: []string{"dash"}, Short: "Dashboard overview"}

	dashboardCmd.AddCommand(&cobra.Command{
		Use:   "stats",
		Short: "Show dashboard stats",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/dashboard/stats")
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	dashboardCmd.AddCommand(&cobra.Command{
		Use:   "overview",
		Short: "Show fleet overview",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/dashboard")
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	dashboardCmd.AddCommand(&cobra.Command{
		Use:   "errors",
		Short: "Show recent errors",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/dashboard/errors")
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	dashboardCmd.AddCommand(&cobra.Command{
		Use:   "activity",
		Short: "Show recent activity",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/dashboard/activity")
			if err != nil {
				return err
			}
			var pretty bytes.Buffer
			json.Indent(&pretty, resp, "", "  ")
			fmt.Println(pretty.String())
			return nil
		},
	})

	dashboardCmd.AddCommand(&cobra.Command{
		Use:   "export",
		Short: "Export all agents as JSON",
		RunE: func(_ *cobra.Command, _ []string) error {
			resp, err := apiGet("/api/v1/dashboard/export")
			if err != nil {
				return err
			}
			fmt.Println(string(resp))
			return nil
		},
	})

	rootCmd.AddCommand(agentCmd, taskCmd, statsCmd, providerCmd, doctorCmd, toolCmd, connCmd, subagentCmd, templateCmd, dashboardCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func patchAgentState(id, state string) error {
	body := map[string]any{"state": state}
	_, err := apiPatch("/api/v1/agents/"+id, body)
	if err != nil {
		return err
	}
	fmt.Printf("Agent %s state: %s\n", id, state)
	return nil
}

func apiGet(path string) ([]byte, error) {
	resp, err := http.Get(apiURL + path)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func apiPost(path string, body any) ([]byte, error) {
	data, _ := json.Marshal(body)
	resp, err := http.Post(apiURL+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func apiPatch(path string, body any) ([]byte, error) {
	data, _ := json.Marshal(body)
	req, err := http.NewRequest("PATCH", apiURL+path, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func apiDelete(path string) ([]byte, error) {
	req, err := http.NewRequest("DELETE", apiURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

var _ = strconv.Itoa
