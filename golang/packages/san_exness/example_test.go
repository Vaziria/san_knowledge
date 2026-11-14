package san_exness_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	exness "github.com/wargasipil/san_exness"
)

// Opening a position: the request is acknowledged at once and finished
// later; WaitOperation (or the Transactions stream) gives the result.
func Example() {
	ctx := context.Background()
	c, err := exness.New(exness.Config{
		APIKey:     os.Getenv("EXN_API_KEY"),
		PrivateKey: os.Getenv("EXN_PRIVATE_KEY"),
		AccountID:  os.Getenv("EXN_ACCOUNT_ID"),
		Gateway:    os.Getenv("EXN_DISCOVERY_BASE_URL"),
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	cond, err := c.Instrument(ctx, "XAUUSDm")
	if err != nil {
		fmt.Println(err)
		return
	}
	ack, err := c.OpenPosition(ctx, exness.OpenPosition{
		Instrument: "XAUUSDm",
		Side:       exness.Buy,
		Volume:     cond.VolumeMin,
		StopLoss:   "2600.000",
	}, "") // an empty key gets a new one: ack.IdempotencyKey, to retry safely
	if err != nil {
		fmt.Println(err)
		return
	}
	wait, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_, err = c.WaitOperation(wait, ack.OperationID, time.Second)
	var refused *exness.OperationError
	if errors.As(err, &refused) && refused.Code == exness.CodeTradingRuleInsufficientMargin {
		fmt.Println("not enough margin")
	}
}
