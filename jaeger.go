package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	// Create the Jaeger exporter
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(url)))
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp),
		// Record information about this application in a Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName("jaeger-test"),
			attribute.String("environment", "localhost"),
			attribute.Int64("ID", 121),
		)),
	)
	return tp, nil
}

func main() {
	fmt.Println("jaeger")

	tp, e := tracerProvider("http://localhost:14268/api/traces")
	fmt.Printf(" ---%#v \n", tp)
	if e != nil {
		log.Fatal(e)
	}

	otel.SetTracerProvider(tp)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer tp.Shutdown(ctx)
	/*
		defer func(ctx context.Context) {
			// Do not make the application hang when it is shutdown.
			ctx, cancel = context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			if err := tp.Shutdown(ctx); err != nil {
				log.Fatal(err)
			}
		}(ctx)
	*/
	tr := tp.Tracer("component-work")

	ctx, span := tr.Start(ctx, "work start")
	defer span.End()

	err := workFunction(ctx)
	if err != nil {
		fmt.Println("work failed")
	}
}

func workFunction(ctx context.Context) error {
	tr := otel.Tracer("component-workfunction")
	ctx1, span := tr.Start(ctx, "WF init")
	defer span.End()

	err := initFunc(ctx1)
	if err != nil {
		return err
	}
	span.End()

	ctx2, span1 := tr.Start(ctx1, "wgroup")
	defer span1.End()

	var wg sync.WaitGroup

	for i := 1; i < 4; i++ {
		wg.Add(1)
		go func(t int, ctx3 context.Context) {
			defer wg.Done()

			tr := otel.Tracer("component-gorutine")
			_, span4 := tr.Start(ctx3, "start gorutine "+string(t))
			defer span4.End()
			span4.AddEvent("waiter", trace.WithAttributes(attribute.String("time", "100")))
			time.Sleep(100 * time.Millisecond)
			span4.AddEvent("waiter2", trace.WithAttributes(attribute.String("time", string(t))))
			time.Sleep(time.Duration(t) * time.Second)
			span4.AddEvent("waiter-print", trace.WithAttributes(attribute.String("print", "message")))

			fmt.Printf("over go rutine %d\n", t)
		}(i, ctx2)
	}

	wg.Wait()

	return nil
}

func initFunc(ctx context.Context) error {
	tr := otel.Tracer("component-init")
	_, span := tr.Start(ctx, "init")
	span.SetAttributes(attribute.Key("init").String("start"))
	defer span.End()

	time.Sleep(10 * time.Millisecond)

	res := rand.Int()
	if res%2 == 0 {
		span.SetAttributes(attribute.Key("result").String("Error"))
		return fmt.Errorf("Bad init")
	}
	span.SetAttributes(attribute.Key("result").String("Success"))
	return nil
}
