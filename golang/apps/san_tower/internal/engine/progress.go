package engine

import (
	"errors"
	"fmt"
	"strconv"

	v1 "github.com/wargasipil/san_tower/gen/san/tower/v1"
)

// Queue Progress (the user, 2026-10-29: "add progress state node, so
// underground flow can chat me 1/2 completed"): how far a batch of a
// workflow's runs has got. A one-at-a-time workflow queues the runs of
// messages sent in a burst; the batch is the runs each queued before the
// one before it ended (store.RunBatch), so it starts again once the queue
// has emptied, and holds after a tower restart.

var progressKinds = []*kind{{
	StepKind: &v1.StepKind{
		Kind: "queue-progress", Title: "Queue Progress", Category: "Data",
		Description: "How far this workflow's batch of runs has got, the runs queued while one before was still going (one at a time): Done counts this run and those of the batch that ended before it, Total adds those still waiting, Left is what waits, and Text is Done/Total, such as 1/2. It counts this run as done: put it where the run's work is over, as before its last message.",
		Outputs:     []*v1.Socket{sock("Text", Text), sock("Done", Text), sock("Total", Text), sock("Left", Text)},
	},
	run: queueProgress,
}}

func queueProgress(c *call) error {
	if c.env.RunBatch == nil || c.env.Run == 0 {
		return errors.New("this run is not recorded, so it has no batch")
	}
	before, waiting, err := c.env.RunBatch(c.env.Workflow, c.env.Run)
	if err != nil {
		return err
	}
	done := before + 1
	total := done + waiting
	c.outputs["Done"] = stringValue(strconv.Itoa(done))
	c.outputs["Total"] = stringValue(strconv.Itoa(total))
	c.outputs["Left"] = stringValue(strconv.Itoa(waiting))
	c.outputs["Text"] = stringValue(fmt.Sprintf("%d/%d", done, total))
	return nil
}
