/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package sbserver

import (
	"context"
	"fmt"
	"time"

	containerstore "github.com/containerd/containerd/pkg/cri/store/container"
	runtime "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// PauseContainer pauses the container.
func (c *criService) PauseContainer(ctx context.Context, r *runtime.PauseContainerRequest) (retRes *runtime.PauseContainerResponse, retErr error) {
	start := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Get container from our container store.
	cntr, err := c.containerStore.Get(r.GetContainerId())
	if err != nil {
		return nil, fmt.Errorf("failed to find container %q in store: %w", r.GetContainerId(), err)
	}
	id := cntr.ID
	meta := cntr.Metadata

	info, err := cntr.Container.Info(ctx)
	if err != nil {
		return nil, fmt.Errorf("get container info: %w", err)
	}

	state := cntr.Status.Get().State()
	if state != runtime.ContainerState_CONTAINER_RUNNING {
		return nil, fmt.Errorf("container is in %s state", criContainerStateToString(state))
	}

	task, err := cntr.Container.Task(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to load task: %w", err)
	}

	if err = task.Pause(ctx); err != nil {
		return nil, fmt.Errorf("failed to pause task %q: %w", id, err)
	}

	// Update container start timestamp.
	if err := cntr.Status.UpdateSync(func(status containerstore.Status) (containerstore.Status, error) {
		status.PausedAt = time.Now().UnixNano()
		return status, nil
	}); err != nil {
		return nil, fmt.Errorf("failed to update container %q state: %w", id, err)
	}
	c.generateAndSendContainerEvent(ctx, id, meta.SandboxID, runtime.ContainerEventType_CONTAINER_PAUSED_EVENT)
	containerPauseTimer.WithValues(info.Runtime.Name).UpdateSince(start)
	return &runtime.PauseContainerResponse{}, nil
}
