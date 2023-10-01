import logging
import multiprocessing
import os
import shutil
import socket
import subprocess
import tempfile
import time
import unittest

import mlflow
import mlflow.server
import python.runfiles.runfiles
import requests

runfiles = python.runfiles.runfiles.Create()

logging.basicConfig(level=logging.INFO)


class TC(unittest.TestCase):
    def start_mlflow_server(self, root_dir) -> str:
        # look for a free port
        sock = socket.socket()
        sock.bind(("", 0))
        port = sock.getsockname()[1]
        sock.close()

        # mlflow server expects gunicorn on its PATH.
        if not shutil.which("gunicorn"):
            gunicorn_path = runfiles.Rlocation("_main/conformance/gunicorn")
            if not os.path.exists(gunicorn_path):
                raise RuntimeError(f"Could not find gunicorn binary at {gunicorn_path}")
            os.environ["PATH"] = os.path.dirname(gunicorn_path) + ":" + os.environ["PATH"]
        logging.info(
            "Starting MLFlow server on port %d with file store %s",
            port,
            root_dir,
        )
        server_process = multiprocessing.Process(
            target=mlflow.server._run_server,
            kwargs={
                "file_store_path": root_dir,
                "registry_store_uri": None,
                "default_artifact_root": None,
                "serve_artifacts": False,
                "artifacts_only": False,
                "artifacts_destination": None,
                "host": "localhost",
                "port": port,
            },
        )
        server_process.start()
        # wait for server to start
        server_uri = f"http://localhost:{port}"
        up = False
        for _ in range(10):
            try:
                up = requests.get(f"{server_uri}/api/2.0/mlflow/experiments/get?experiment_id=0")
                break
            except requests.exceptions.ConnectionError:
                time.sleep(0.5)
        self.assertTrue(up, f"server did not start: {server_uri}")
        self.assertTrue(server_process.is_alive())
        self.addCleanup(server_process.terminate)
        return server_uri

    def test_conformance(self):
        for binary in (
            "_main/conformance/go_/go",
            "_main/conformance/py",
        ):
            lang = os.path.basename(binary)
            with self.subTest(lang=lang):
                for start_server in (False, True):
                    root_dir = tempfile.mkdtemp()
                    # Python mlflow client fails if the directory
                    # exists but does not already contain the default experiment.
                    # Remove it so that it creates the default experiment rather than failing.
                    os.rmdir(root_dir)
                    with self.subTest(scheme="http" if start_server else "file"):
                        server_process = None
                        if start_server:
                            server_uri = self.start_mlflow_server(root_dir)
                            env = {"MLFLOW_TRACKING_URI": server_uri}
                        else:
                            env = {"MLFLOW_TRACKING_URI": root_dir}
                        subprocess.check_call(
                            (runfiles.Rlocation(binary),),
                            env=env,
                        )

                        client = mlflow.tracking.MlflowClient(
                            tracking_uri=env["MLFLOW_TRACKING_URI"]
                        )
                        exp = client.get_experiment_by_name("Default")
                        runs = client.search_runs([exp.experiment_id])
                        self.assertEqual(len(runs), 1)
                        run = runs[0]
                        metric_key = "metric0"
                        tag_key = "tag0"
                        param_key = "param0"

                        self.assertIn(metric_key, run.data.metrics)
                        self.assertEqual(run.data.metrics[metric_key], 10.0)
                        self.assertIn(tag_key, run.data.tags)
                        self.assertEqual(run.data.tags[tag_key], "value0")
                        self.assertIn(param_key, run.data.params)
                        self.assertEqual(run.data.params[param_key], "value0")

                        artifacts = client.list_artifacts(run.info.run_id)
                        self.assertEqual(len(artifacts), 1)
                        self.assertEqual(artifacts[0].path, "artifact0.txt")


if __name__ == "__main__":
    unittest.main()
