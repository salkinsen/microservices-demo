/*
 * Copyright The OpenTelemetry Authors
 * SPDX-License-Identifier: Apache-2.0
 */

// The following code has been taken and adapted from
// https://github.com/open-telemetry/opentelemetry-java/blob/v1.3.0/examples/grpc/src/main/java/io/opentelemetry/example/grpc/ExampleConfiguration.java
package hipstershop;

import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.api.trace.propagation.W3CTraceContextPropagator;
import io.opentelemetry.context.propagation.ContextPropagators;
import io.opentelemetry.exporter.logging.LoggingSpanExporter;
import io.opentelemetry.sdk.OpenTelemetrySdk;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.opentelemetry.sdk.trace.export.SimpleSpanProcessor;
import io.opentelemetry.exporter.jaeger.thrift.JaegerThriftSpanExporter;
import io.opentelemetry.sdk.resources.Resource;
import io.opentelemetry.api.common.Attributes;
import io.opentelemetry.semconv.resource.attributes.ResourceAttributes;
import io.opentelemetry.sdk.trace.export.BatchSpanProcessor;


class OtelConfig {

  static OpenTelemetry initOpenTelemetry() {

    String jaegerAddr = System.getenv("JAEGER_SERVICE_ADDR");

    JaegerThriftSpanExporter exporter =
        JaegerThriftSpanExporter.builder()
            .setEndpoint(jaegerAddr)
            .build();
    
    Resource serviceNameResource =
    Resource.create(Attributes.of(ResourceAttributes.SERVICE_NAME, "adservice"));

    SdkTracerProvider sdkTracerProvider =
        SdkTracerProvider.builder()
            .addSpanProcessor(BatchSpanProcessor.builder(exporter).build())
            .setResource(Resource.getDefault().merge(serviceNameResource))
            .build();

    OpenTelemetrySdk openTelemetrySdk =
        OpenTelemetrySdk.builder()
            .setTracerProvider(sdkTracerProvider)
            // install the W3C Trace Context propagator
            .setPropagators(ContextPropagators.create(W3CTraceContextPropagator.getInstance()))
            .build();

    // add shutdown hook for process: shutdown SDK
    Runtime.getRuntime()
        .addShutdownHook(
            new Thread(
                () -> {
                  System.err.println(
                      "*** forcing the Span Exporter to shutdown and process the remaining spans");
                  sdkTracerProvider.shutdown();
                  System.err.println("*** Trace Exporter shut down");
                }));

    return openTelemetrySdk;
  }
}