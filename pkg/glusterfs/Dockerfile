# Copyright 2018 The Gluster CSI Authors.

# Licensed under GNU LESSER GENERAL PUBLIC LICENSE Version 3, 29 June 2007
# You may obtain a copy of the License at
#    https://opensource.org/licenses/lgpl-3.0.html

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM centos:7.5.1804

# Copy glusterfs-csi-driver from build directory
COPY ./build/glusterfs-csi-driver /glusterfs-csi-driver

RUN yum -y install glusterfs-fuse && yum clean all -y

ENTRYPOINT ["/glusterfs-csi-driver"]
