package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
	"go.uber.org/zap"

	"github.com/log-analytics/server/internal/alert"
	"github.com/log-analytics/server/internal/api"
	"github.com/log-analytics/server/internal/config"
	"github.com/log-analytics/server/internal/detection"
	"github.com/log-analytics/server/internal/domain"
	es "github.com/log-analytics/server/internal/elasticsearch"
	"github.com/log-analytics/server/internal/kafka"
	"github.com/log-analytics/server/internal/pipeline"
	"github.com/log-analytics/server/internal/smtp"
)

// runPipeline wires all pipeline components and runs them until ctx is cancelled
// or a fatal error occurs. It returns nil if the only error is context.Canceled.
func runPipeline(ctx context.Context, cfg config.Config, logger *zap.Logger) error {
	// 1. Elasticsearch setup (fail-fast)
	esClient, err := es.NewClient(es.ClientConfig{
		Addresses:       cfg.Elasticsearch.Addresses,
		Username:        cfg.Elasticsearch.Username,
		Password:        cfg.Elasticsearch.Password,
		MaxIdleConns:    cfg.Elasticsearch.MaxIdleConns,
		ResponseTimeout: cfg.Elasticsearch.ResponseTimeout,
	})
	if err != nil {
		return err
	}

	if err := es.ApplyIndexTemplates(ctx, esClient); err != nil {
		return err
	}

	logIndexer, err := es.NewLogIndexer(esClient, logger)
	if err != nil {
		return err
	}

	anomalyIndexer, err := es.NewAnomalyIndexer(esClient, logger)
	if err != nil {
		return err
	}

	// 2. Kafka consumer
	consumer, err := kafka.New(cfg.Kafka, logger)
	if err != nil {
		return err
	}

	// 3. Detection engine — build all rules from config
	offHoursRule, err := detection.NewOffHoursAccessRule(cfg.Detection.Rules.OffHours)
	if err != nil {
		return err
	}

	detectors := []domain.Detector{
		detection.NewErrorRateRule(cfg.Detection.Rules.ErrorRate),
		detection.NewLatencyThresholdRule(cfg.Detection.Rules.Latency),
		detection.NewRepeatedFailureRule(cfg.Detection.Rules.RepeatedFailure),
		detection.NewAuthFailureBurstRule(cfg.Detection.Rules.AuthBurst),
		offHoursRule,
		detection.NewServiceSilenceRule(cfg.Detection.Rules.ServiceSilence),
	}

	engine := detection.NewDetectorEngine(detectors, cfg.Detection, logger)

	// 4. SMTP alerter
	smtpCfg := smtp.SMTPConfig{
		Host:       cfg.SMTP.Host,
		Port:       cfg.SMTP.Port,
		Username:   cfg.SMTP.Username,
		Password:   cfg.SMTP.Password,
		From:       cfg.SMTP.From,
		Recipients: cfg.SMTP.Recipients,
		TLSPolicy:  cfg.SMTP.TLSPolicy,
	}

	alerter, err := smtp.NewSMTPAlerter(smtpCfg, logger)
	if err != nil {
		return err
	}

	// 5. Alert dispatcher
	dispatcher := alert.NewDispatcher(engine.Anomalies(), anomalyIndexer, alerter, logger)

	// 6. API server
	apiServer := api.NewServer(logIndexer, anomalyIndexer, esClient, logger, cfg.API.Port)

	// 7. Wire errgroup goroutines
	g, gCtx := errgroup.WithContext(ctx)

	// Kafka consumer
	g.Go(func() error {
		return consumer.Run(gCtx)
	})

	// Pipeline worker: parse → index log → evaluate for anomalies
	g.Go(func() error {
		for msg := range consumer.Messages() {
			entry, err := pipeline.Parse(msg)
			if err != nil {
				logger.Warn("parse error, using fallback entry", zap.Error(err))
			}
			if indexErr := logIndexer.IndexLog(gCtx, entry); indexErr != nil {
				logger.Warn("index log error", zap.Error(indexErr))
			}
			engine.Evaluate(entry)
		}
		return nil
	})

	// Anomaly dispatcher
	g.Go(func() error {
		return dispatcher.Run(gCtx)
	})

	// SMTP alerter
	g.Go(func() error {
		alerter.Run(gCtx)
		return nil
	})

	// API server
	g.Go(func() error {
		if err := apiServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	})

	// Shutdown watcher — LIFO order
	g.Go(func() error {
		<-gCtx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		if err := apiServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("api server shutdown error", zap.Error(err))
		}
		engine.Stop()
		if err := logIndexer.Close(shutdownCtx); err != nil {
			logger.Warn("log indexer close error", zap.Error(err))
		}
		if err := anomalyIndexer.Close(shutdownCtx); err != nil {
			logger.Warn("anomaly indexer close error", zap.Error(err))
		}
		return nil
	})

	// 8. Mark API server ready after all setup
	apiServer.SetReady()

	// 9. Wait for all goroutines; filter out context.Canceled
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}
	return nil
}
