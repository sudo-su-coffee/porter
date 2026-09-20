#!/bin/bash
cd /tmp/sample-app
docker buildx build --output type=oci,dest=/tmp/sample-app/image.oci.tar . > /tmp/ocibuild.log 2>&1
echo "DONE-RC:$?" >> /tmp/ocibuild.log
