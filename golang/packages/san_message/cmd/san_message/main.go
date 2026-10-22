// Command san_message runs San Message as a standalone server, and manages
// the topics and subscribers of a running one.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/urfave/cli/v3"

	"github.com/wargasipil/san_message"
	v1 "github.com/wargasipil/san_message/gen/san/message/v1"
	"github.com/wargasipil/san_message/gen/san/message/v1/messagev1connect"
)

// version is stamped at build time.
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := root(os.Stdout).Run(ctx, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "san_message: %v\n", err)
		os.Exit(1)
	}
}

func root(out io.Writer) *cli.Command {
	return &cli.Command{
		Name:    "san_message",
		Usage:   "lightweight message broker",
		Version: version,
		Writer:  out,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "debug, info, warn or error",
				Value:   "info",
				Sources: cli.EnvVars("SAN_MESSAGE_LOG_LEVEL"),
			},
			&cli.StringFlag{
				Name:    "server",
				Usage:   "URL of the running server that topic and subscriber commands manage",
				Value:   "http://127.0.0.1:7070",
				Sources: cli.EnvVars("SAN_MESSAGE_SERVER"),
			},
		},
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			var level slog.Level
			if err := level.UnmarshalText([]byte(cmd.String("log-level"))); err != nil {
				return ctx, fmt.Errorf("bad --log-level %q", cmd.String("log-level"))
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
			return ctx, nil
		},
		Commands: []*cli.Command{serverCommand(), topicCommand(), subscriberCommand()},
	}
}

// ---------------------------------------------------------------- server ---

func serverCommand() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "run the broker",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "listen", Value: "127.0.0.1:7070", Usage: "address to listen on"},
			&cli.StringFlag{Name: "data", Value: "event_data", Usage: "data folder"},
		},
		Action: runServer,
	}
}

func runServer(ctx context.Context, cmd *cli.Command) error {
	log := slog.Default()
	b, err := san_message.Open(san_message.Options{Dir: cmd.String("data"), Log: log})
	if err != nil {
		return err
	}
	defer b.Close()

	mux := http.NewServeMux()
	mux.Handle(b.Handler())
	srv := &http.Server{Addr: cmd.String("listen"), Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	san_message.ConfigureServer(srv)

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("san_message listening", "addr", srv.Addr, "data", cmd.String("data"), "version", version)

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// Close the broker first: it ends the open Subscribe streams, which a
	// graceful shutdown would otherwise wait on until its timeout.
	closeErr := b.Close()
	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return errors.Join(closeErr, srv.Shutdown(sctx))
}

// ----------------------------------------------------------------- topic ---

func topicCommand() *cli.Command {
	return &cli.Command{
		Name:  "topic",
		Usage: "manage the topics of a running server",
		Commands: []*cli.Command{
			{
				Name:      "create",
				Usage:     "create a topic (letters, digits, - and _)",
				ArgsUsage: "<topic>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					key, err := oneArg(cmd, "topic")
					if err != nil {
						return err
					}
					if _, err := client(cmd).CreateTopic(ctx, connect.NewRequest(&v1.CreateTopicRequest{Topic: key})); err != nil {
						return rpcError(err)
					}
					fmt.Fprintf(cmd.Root().Writer, "created topic %s\n", key)
					return nil
				},
			},
			{
				Name:      "delete",
				Usage:     "delete a topic and its log files (refused while it has subscribers)",
				ArgsUsage: "<topic>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					key, err := oneArg(cmd, "topic")
					if err != nil {
						return err
					}
					if _, err := client(cmd).DeleteTopic(ctx, connect.NewRequest(&v1.DeleteTopicRequest{Topic: key})); err != nil {
						return rpcError(err)
					}
					fmt.Fprintf(cmd.Root().Writer, "deleted topic %s\n", key)
					return nil
				},
			},
			{
				Name:   "list",
				Usage:  "list topics",
				Action: listTopics,
			},
		},
	}
}

func listTopics(ctx context.Context, cmd *cli.Command) error {
	resp, err := client(cmd).ListTopics(ctx, connect.NewRequest(&v1.ListTopicsRequest{}))
	if err != nil {
		return rpcError(err)
	}
	out := cmd.Root().Writer
	if len(resp.Msg.GetTopics()) == 0 {
		fmt.Fprintln(out, "No topics.")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TOPIC\tLAST EVENT ID\tSEGMENT\tOFFSET\tSUBSCRIBERS")
	for _, t := range resp.Msg.GetTopics() {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n", t.GetKey(), t.GetLastEventId(), t.GetSegment(), t.GetOffset(), t.GetSubscribers())
	}
	return w.Flush()
}

// ------------------------------------------------------------ subscriber ---

func subscriberCommand() *cli.Command {
	return &cli.Command{
		Name:  "subscriber",
		Usage: "manage the subscribers of a running server",
		Commands: []*cli.Command{
			{
				Name:      "create",
				Usage:     "create a subscriber; it gets only events published after it",
				ArgsUsage: "<subscriber>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "topic", Required: true, Usage: "topic it reads"},
					&cli.StringFlag{Name: "type", Required: true, Usage: "webhook or pull"},
					&cli.StringFlag{Name: "webhook-uri", Usage: "http or https URL events are POSTed to (webhook type)"},
				},
				Action: createSubscriber,
			},
			{
				Name:      "delete",
				Usage:     "delete a subscriber",
				ArgsUsage: "<subscriber>",
				Action: func(ctx context.Context, cmd *cli.Command) error {
					key, err := oneArg(cmd, "subscriber")
					if err != nil {
						return err
					}
					if _, err := client(cmd).DeleteSubscriber(ctx, connect.NewRequest(&v1.DeleteSubscriberRequest{Subscriber: key})); err != nil {
						return rpcError(err)
					}
					fmt.Fprintf(cmd.Root().Writer, "deleted subscriber %s\n", key)
					return nil
				},
			},
			{
				Name:  "list",
				Usage: "list subscribers",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "topic", Usage: "only this topic's subscribers"},
				},
				Action: listSubscribers,
			},
		},
	}
}

func createSubscriber(ctx context.Context, cmd *cli.Command) error {
	key, err := oneArg(cmd, "subscriber")
	if err != nil {
		return err
	}
	var typ v1.SubscriberType
	switch t := cmd.String("type"); t {
	case "webhook":
		typ = v1.SubscriberType_SUBSCRIBER_TYPE_WEBHOOK
	case "pull":
		typ = v1.SubscriberType_SUBSCRIBER_TYPE_PULL
	default:
		return fmt.Errorf("--type must be webhook or pull, got %q", t)
	}
	req := &v1.CreateSubscriberRequest{
		Subscriber: key,
		Topic:      cmd.String("topic"),
		Type:       typ,
		WebhookUri: cmd.String("webhook-uri"),
	}
	if _, err := client(cmd).CreateSubscriber(ctx, connect.NewRequest(req)); err != nil {
		return rpcError(err)
	}
	fmt.Fprintf(cmd.Root().Writer, "created %s subscriber %s on topic %s\n", cmd.String("type"), key, req.Topic)
	return nil
}

func listSubscribers(ctx context.Context, cmd *cli.Command) error {
	resp, err := client(cmd).ListSubscribers(ctx, connect.NewRequest(&v1.ListSubscribersRequest{Topic: cmd.String("topic")}))
	if err != nil {
		return rpcError(err)
	}
	out := cmd.Root().Writer
	if len(resp.Msg.GetSubscribers()) == 0 {
		fmt.Fprintln(out, "No subscribers.")
		return nil
	}
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SUBSCRIBER\tTOPIC\tTYPE\tSEGMENT\tOFFSET\tRETRY\tCONNECTED\tWEBHOOK URI")
	for _, s := range resp.Msg.GetSubscribers() {
		typ, connected := "webhook", "-"
		if s.GetType() == v1.SubscriberType_SUBSCRIBER_TYPE_PULL {
			typ, connected = "pull", "no"
			if s.GetConnected() {
				connected = "yes"
			}
		}
		retry := "-"
		if s.GetRetryCount() > 0 {
			retry = fmt.Sprintf("%d, next %s", s.GetRetryCount(), time.UnixMilli(s.GetRetryAtMs()).Format(time.TimeOnly))
		}
		uri := s.GetWebhookUri()
		if uri == "" {
			uri = "-"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%d\t%s\t%s\t%s\n",
			s.GetKey(), s.GetTopic(), typ, s.GetSegment(), s.GetOffset(), retry, connected, uri)
	}
	return w.Flush()
}

// ---------------------------------------------------------------- shared ---

func client(cmd *cli.Command) messagev1connect.MessageServiceClient {
	return messagev1connect.NewMessageServiceClient(http.DefaultClient, cmd.String("server"))
}

func oneArg(cmd *cli.Command, name string) (string, error) {
	if cmd.Args().Len() != 1 {
		return "", fmt.Errorf("want one <%s> argument, got %d", name, cmd.Args().Len())
	}
	return cmd.Args().First(), nil
}

// rpcError keeps the server's own message, without connect's code prefix.
func rpcError(err error) error {
	if ce := new(connect.Error); errors.As(err, &ce) {
		return errors.New(ce.Message())
	}
	return err
}
