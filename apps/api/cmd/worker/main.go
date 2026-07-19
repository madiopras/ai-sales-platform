// Command worker runs periodic maintenance jobs for the payment domain.
//
// Currently it expires invoices whose expired_at has passed and releases the
// reserved stock back to the catalog (BR-036). Run it as a cron/systemd timer:
//
//	worker            # single pass, then exit (good for cron)
//	worker -loop      # run continuously, sweeping every -interval
//
// It reuses the same DI container as the API so config + DB wiring stay in one
// place.
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prasdios/ai-sales-platform/apps/api/internal/container"
	"go.uber.org/zap"
)

func main() {
	loop := flag.Bool("loop", false, "run continuously instead of a single pass")
	interval := flag.Duration("interval", time.Minute, "sweep interval when running with -loop")
	batch := flag.Int("batch", 100, "max invoices to expire per pass")
	flag.Parse()

	ctr, err := container.New()
	if err != nil {
		panic(err)
	}
	defer ctr.Close()
	log := ctr.Logger

	autoCompleteAfter := ctr.Config.Biteship.AutoCompleteAfter

	sweep := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Expire unpaid invoices and release their reserved stock (BR-036).
		count, err := ctr.PaymentService.ExpireInvoices(ctx, *batch)
		if err != nil {
			log.Error("expire invoices failed", zap.Error(err))
		} else if count > 0 {
			log.Info("expired invoices", zap.Int("count", count))
		}

		// Auto-complete orders that have been delivered long enough (BR-034).
		completed, err := ctr.ShippingService.CompleteDeliveredOrders(ctx, autoCompleteAfter, *batch)
		if err != nil {
			log.Error("auto-complete delivered orders failed", zap.Error(err))
		} else if completed > 0 {
			log.Info("auto-completed delivered orders", zap.Int("count", completed))
		}
	}

	if !*loop {
		sweep()
		return
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Info("payment worker started", zap.Duration("interval", *interval))
	sweep() // run once immediately on boot
	for {
		select {
		case <-ticker.C:
			sweep()
		case <-quit:
			log.Info("payment worker stopping")
			return
		}
	}
}
