#!/bin/bash -e
#
# Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO111MODULE=on \
IMPORTS_PATH="$(dirname $0)/../../dev/imports.yaml" \
OPERATION="${1:-RECONCILE}" \
  go run \
    -mod=vendor \
    -ldflags "$LD_FLAGS" \
    "$(dirname $0)"/../../cmd/virtual-garden/main.go \
    --handle-namespace=true
