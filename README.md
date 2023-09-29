# mlflow-go

Go MLFlow client.

File-based storage based on Python
[mlflow.store.tracking.file_store](https://github.com/mlflow/mlflow/blob/8cd2eb0f7975decefb88af60ac5cc4f968458ab3/mlflow/store/tracking/file_store.py).

HTTP / REST clients based on Python
[mlflow.store.tracking.rest_store](https://github.com/mlflow/mlflow/blob/47ee67190d20e93103ec4c4ba6f5350fb8dbb7fa/mlflow/store/tracking/rest_store.py).

## Protocol Buffers

The [protos](protos) directory is copied from the official mlflow repo.
To update it, run [update_protos.sh](update_protos.sh).

