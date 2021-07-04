// if(process.env.DISABLE_TRACING) {
//   console.log("Tracing disabled.")
//   return
// }

// console.log("Tracing enabled")

// TODO: remove http instrumentation, (if grpc works without it)
// (TODO: remove uncesseary dependencies)
// TODO: use env variable for jaeger agent

// The following code was taken (and adapted) from this official getting-started-guide:
// https://github.com/open-telemetry/opentelemetry-js/blob/main/getting-started/README.md

'use strict';

const { diag, DiagConsoleLogger, DiagLogLevel } = require("@opentelemetry/api");
const { NodeTracerProvider } = require("@opentelemetry/node");
const { Resource } = require('@opentelemetry/resources');
const { ResourceAttributes } = require('@opentelemetry/semantic-conventions');
const { BatchSpanProcessor } = require("@opentelemetry/tracing");
const { JaegerExporter } = require("@opentelemetry/exporter-jaeger");
const { registerInstrumentations } = require("@opentelemetry/instrumentation");
const { HttpInstrumentation } = require("@opentelemetry/instrumentation-http");
const { GrpcInstrumentation } = require("@opentelemetry/instrumentation-grpc");

const provider = new NodeTracerProvider({
  resource: new Resource({
    [ResourceAttributes.SERVICE_NAME]: "currencyservice",
  })
});

// uncomment to get logs about tracing
// diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.ALL);

provider.addSpanProcessor(
  new BatchSpanProcessor(
    new JaegerExporter({
      host: 'jaeger-agent',
      port: 6832
    })
  )
);

provider.register();

registerInstrumentations({
  instrumentations: [
    new HttpInstrumentation(),
    new GrpcInstrumentation(),
  ],
});

console.log("tracing initialized");