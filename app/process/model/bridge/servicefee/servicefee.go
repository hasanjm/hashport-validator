/*
 * Copyright 2021 LimeChain Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package servicefee

import (
	"math/big"
	"sync"
)

type Servicefee struct {
	serviceFee big.Int
	mutex      sync.RWMutex
}

func (s *Servicefee) Get() *big.Int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return &s.serviceFee
}

func (s *Servicefee) Set(serviceFee big.Int) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	s.serviceFee = serviceFee
}
