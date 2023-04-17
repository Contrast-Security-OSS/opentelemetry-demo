# Overview

This repo is a fork of opentelemetry-demo with some experimental features. The
normal and Contrast service images are all stored in Contrast's GitHub
Container Repository.  To access our private `ghcr.io/contrast-security-inc`,
you will need a GitHub PAT token and will need to login to the registry before
attempting to pull any of these images.

# Docker Login for ghcr.io
Obtain a "classic" personal token from GitHub with the `read:packages` scope.

After obtaining your token, run:
```console
export CR_PAT=YOUR_TOKEN
```
```console
echo $CR_PAT | docker login ghcr.io -u USERNAME --password-stdin
```
You should see this response returned:
```console
> Login Succeeded
```

You are now ready run the docker commands

# Starting the lab/demo

From the top level repo directory, run
```console
docker compose up --no-build
```

This will pull down all the images for the lab/demo environment and run them.
It will likely take about 3 minutes for the entire system to be pulled down,
started, and ready to accept requests.

# Accessing Stuff

Here are the links for various components offered from the lab/demo
environment.

Webstore: <http://localhost:8080/>
Grafana: <http://localhost:8080/grafana/>
Feature Flags UI: <http://localhost:8080/feature/>
Load Generator UI: <http://localhost:8080/loadgen/>
Jaeger UI: <http://localhost:8080/jaeger/ui/>

Contrast ObsData (GraphiQl) Api: <http://localhost:8080/obsdata/graphiql?path=/obsdata/graphql>
Contrast ObsData (REST) API: <http://localhost:8080/obsdata/>

# Appendix

[Working with ghcr.io](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
[Obtain a classic github token](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)
[Upstream OTEL Demo Documentation](https://opentelemetry.io/docs/demo/)
