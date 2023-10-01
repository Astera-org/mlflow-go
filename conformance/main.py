import os
import tempfile

import mlflow


def main():
    for i in range(10):
        mlflow.log_metric("metric0", i + 1, step=i)

    mlflow.set_tag("tag0", "value0")
    mlflow.log_param("param0", "value0")
    temp_dir = tempfile.mkdtemp()
    artifact_path = os.path.join(temp_dir, "artifact0.txt")
    with open(artifact_path, "wt") as f:
        f.write("hello\n")
    mlflow.log_artifact(artifact_path)


if __name__ == "__main__":
    main()
