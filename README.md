# mlflow-go

Go MLFlow client.

Supports the tracking API, with local files and HTTP.

## Usage

The API is modeled after the official Python client, so the [official MLFlow docs](https://mlflow.org/docs/latest/tracking.html) may be useful.

Authentication to Databricks-hosted MLFlow is only supported via access token, not via Databricks username and password.
Follow the [instructions](https://docs.databricks.com/dev-tools/api/latest/authentication.html#generate-a-personal-access-token)
to get a personal access token.

## Development

Install Bazel using [Bazelisk](https://github.com/bazelbuild/bazelisk/blob/master/README.md).

You can install Go on your own or use the version that Bazel downloads.
After a `bazel test //...`, you should be able to find the Go binary like so:

```sh
find -L bazel-mlflow-go/external -wholename "*/bin/go"
```

### Manual tests

There are some tests that assume something about the environment.
They can be run with `go test -tags manual`, or by specifying the exact
target to `bazel test`. When making changes to the code that is not well-covered by
the unit tests, please run the manual tests.

### Protocol Buffers

The .proto files in the [protos](protos) directory are copied from the official mlflow repo.
Unfortunately not everybody uses Bazel, and so we have to check in the generated
protocol buffer code.
To download the latest .proto files and regenerate the .pb.go files, run
[update_protos.sh](update_protos.sh).

