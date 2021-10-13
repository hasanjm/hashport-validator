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

package router

import "errors"

var errorDivisionByZero = errors.New("division by zero")

// HasEnoughSignatures checks whether the amount of signatures is enough for submission to Router contract
func HasEnoughSignatures(members, membersPercentage, membersPrecision, signatures int64) (bool, error) {
	if membersPrecision == 0 {
		return false, errorDivisionByZero
	}
	multipliedMembers := members * membersPercentage
	requiredSigCount := multipliedMembers / membersPrecision
	if multipliedMembers%membersPrecision != 0 {
		requiredSigCount++
	}

	return signatures >= requiredSigCount, nil
}
