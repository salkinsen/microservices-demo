if(process.env.DISABLE_TRACING) {
  console.log("Tracing disabled.")
  return
}

console.log("Tracing enabled")

// TODO: add tracing setup, see:
// https://github.com/open-telemetry/opentelemetry-js/blob/main/getting-started/README.md