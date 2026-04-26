package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jyzs_proxy_poc/internal/admin"
	"jyzs_proxy_poc/internal/processor"
	"jyzs_proxy_poc/internal/proxy"
	"jyzs_proxy_poc/internal/reco"
	"jyzs_proxy_poc/internal/rules"
	"jyzs_proxy_poc/internal/storage"
)

func main() {
	var (
		proxyListen       = flag.String("listen", "127.0.0.1:18080", "proxy listen address")
		adminListen       = flag.String("admin-listen", "127.0.0.1:18081", "admin HTTP listen address")
		dbPath            = flag.String("db", "data/proxy_poc.db", "sqlite database file path")
		rulesPath         = flag.String("rules", "config/rules.json", "rules file path")
		maxBodyKB         = flag.Int("max-body-kb", 512, "max request/response body capture size in KB")
		enableMITM        = flag.Bool("mitm", false, "enable HTTPS MITM inspection")
		exportCAPath      = flag.String("export-ca", "", "optional path to export MITM CA certificate")
		windowSize        = flag.Int("window-size", 30, "number of recent prices used as baseline window")
		minSamples        = flag.Int("min-samples", 8, "minimum samples before leak recommendation")
		discountThreshold = flag.Float64("discount-threshold", 0.2, "recommend when price <= baseline*(1-threshold)")
	)
	flag.Parse()

	ruleSet, err := rules.LoadRuleSet(*rulesPath)
	if err != nil {
		log.Fatalf("load rules failed: %v", err)
	}

	db, err := storage.NewSQLiteStore(*dbPath)
	if err != nil {
		log.Fatalf("open sqlite failed: %v", err)
	}
	defer db.Close()

	recoEngine := reco.Engine{
		WindowSize:        *windowSize,
		MinSamples:        *minSamples,
		DiscountThreshold: *discountThreshold,
	}
	captureProcessor := processor.New(db, recoEngine)

	adminSrv := admin.NewServer(*adminListen, db)
	go func() {
		log.Printf("admin API listening on http://%s", *adminListen)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("admin server stopped with error: %v", err)
		}
	}()

	proxySrv, err := proxy.NewServer(proxy.Config{
		ListenAddr:   *proxyListen,
		MaxBodyBytes: int64(*maxBodyKB) * 1024,
		EnableMITM:   *enableMITM,
		ExportCAPath: *exportCAPath,
		RuleSet:      ruleSet,
		Processor:    captureProcessor,
	})
	if err != nil {
		log.Fatalf("create proxy server failed: %v", err)
	}

	go func() {
		log.Printf("proxy listening on %s (mitm=%v)", *proxyListen, *enableMITM)
		if err := proxySrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("proxy server stopped with error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = proxySrv.Shutdown(ctx)
	_ = adminSrv.Shutdown(ctx)
	log.Println("shutdown complete")
}

