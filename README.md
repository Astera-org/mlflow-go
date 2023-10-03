# mlflow-go

Go [MLFlow](https://mlflow.org) client.

Supports the Tracking API, with local files and HTTP.

## Usage

See the examples in [conformance/main.go](conformance/main.go), or fully
rendered documentation on [pkg.go.dev](https://pkg.go.dev/github.com/Astera-org/mlflow-go).

## Development

Install Bazel using [Bazelisk](https://github.com/bazelbuild/bazelisk/blob/master/README.md).
Some tests require Bazel to run (i.e. they are *not* run by `go test`).

If you want to use the `go` tool instead of / in addition to Bazel, you can install Go on your
own or use the version that Bazel downloads.

After a `bazel test //...`, you should be able to find the Go binary like so:

```sh
find -L bazel-mlflow-go/external -wholename "*/bin/go"
```

Install [pre-commit](https://pre-commit.com/).

### Manual tests

There are some tests that assume something about the environment.
They can be run with `go test -tags manual`, or by specifying the exact
target to `bazel test`. When making changes to the code that is not well-covered by
the unit tests, please run the manual tests.

You can list the manual tests with:

```sh
bazel query "attr(tags, '\\bmanual\\b', //...)"
```
