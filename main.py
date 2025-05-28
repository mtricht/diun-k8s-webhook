from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from kubernetes import client, config
from datetime import datetime
import uvicorn
import logging
import os

app = FastAPI()
logging.basicConfig(level=logging.INFO)

class DiunMetadata(BaseModel):
    pod_name: str
    pod_namespace: str

class DiunWebhook(BaseModel):
    metadata: DiunMetadata

def restart_deployment(metadata: DiunMetadata):
    try:
        config.load_incluster_config()
    except config.ConfigException:
        config.load_kube_config()

    v1 = client.CoreV1Api()
    apps_v1 = client.AppsV1Api()

    pod = v1.read_namespaced_pod(metadata.pod_name, metadata.pod_namespace)
    owner_refs = pod.metadata.owner_references
    if not owner_refs or owner_refs[0].kind != "ReplicaSet":
        logging.warning("Pod not controlled by ReplicaSet")
        return

    replicaset_name = owner_refs[0].name
    rs = apps_v1.read_namespaced_replica_set(replicaset_name, metadata.pod_namespace)
    rs_owner_refs = rs.metadata.owner_references
    if not rs_owner_refs or rs_owner_refs[0].kind != "Deployment":
        logging.warning("ReplicaSet not controlled by Deployment")
        return

    deployment_name = rs_owner_refs[0].name

    now = datetime.now(datetime.UTC).strftime("%Y%m%d%H%M%S")
    patch = {
        "spec": {
            "template": {
                "metadata": {
                    "annotations": {
                        "kubectl.kubernetes.io/restartedAt": now
                    }
                }
            }
        }
    }
    try:
        apps_v1.patch_namespaced_deployment(
            name=deployment_name,
            namespace=metadata.pod_namespace,
            body=patch
        )
        logging.info(f"Deployment {deployment_name} in namespace {metadata.pod_namespace} restarted.")
    except Exception as e:
        logging.error(f"Failed to patch deployment: {e}")

@app.post("/webhook")
async def webhook(payload: dict):
    try:
        if "metadata" in payload:
            metadata = payload["metadata"]
        else:
            raise HTTPException(status_code=400, detail="Missing metadata in payload")

        pod_name = metadata.get("pod_name")
        pod_namespace = metadata.get("pod_namespace")
        if not pod_name or not pod_namespace:
            raise HTTPException(status_code=400, detail="Missing pod_name or pod_namespace in metadata")

        restart_deployment(DiunMetadata(pod_name=pod_name, pod_namespace=pod_namespace))
        return {"status": "ok"}
    except Exception as e:
        logging.error(f"Error handling webhook: {e}")
        raise HTTPException(status_code=500, detail=str(e))

if __name__ == "__main__":
    port = int(os.environ.get("PORT", 8080))
    uvicorn.run("main:app", host="0.0.0.0", port=port)
