# Protocol Buffers

The .proto files in this directory are copied from the official mlflow repo.

Unfortunately not everybody uses Bazel, and so we have to check in the generated
protocol buffer Go code.
To download the latest .proto files and regenerate the .pb.go files, run
[update_protos.sh](/tools/update_protos.sh).

These were copied from MLFlow version v2.7.1.
