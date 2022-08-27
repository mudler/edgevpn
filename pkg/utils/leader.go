/*
Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package utils

import "hash/fnv"

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func Leader(actives []string) string {
	// first get available nodes
	leaderboard := map[string]uint32{}

	leader := actives[0]

	// Compute who is leader at the moment
	for _, a := range actives {
		leaderboard[a] = hash(a)
		if leaderboard[leader] < leaderboard[a] {
			leader = a
		}
	}
	return leader
}
