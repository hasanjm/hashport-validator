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

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_DivisionByZero(t *testing.T) {
	members := int64(3)
	percentage := int64(50)
	precision := int64(0)
	signatures := int64(1)

	result, err := HasEnoughSignatures(members, percentage, precision, signatures)

	assert.Error(t, err, errorDivisionByZero)
	assert.False(t, result)
}

func Test_NotEnoughSignatures_NoRemainder(t *testing.T) {
	members := int64(3)
	percentage := int64(100)
	precision := int64(100)
	signatures := int64(1)

	result, err := HasEnoughSignatures(members, percentage, precision, signatures)

	assert.Nil(t, err)
	assert.False(t, result)
}

func Test_NotEnoughSignatures(t *testing.T) {
	members := int64(3)
	percentage := int64(51)
	precision := int64(100)
	signatures := int64(1)

	result, err := HasEnoughSignatures(members, percentage, precision, signatures)

	assert.Nil(t, err)
	assert.False(t, result)
}

func Test_EnoughSignatures_NoRemainder(t *testing.T) {
	members := int64(3)
	percentage := int64(100)
	precision := int64(100)
	signatures := int64(3)

	result, err := HasEnoughSignatures(members, percentage, precision, signatures)

	assert.Nil(t, err)
	assert.True(t, result)
}

func Test_EnoughSignatures(t *testing.T) {
	members := int64(3)
	percentage := int64(51)
	precision := int64(100)
	signatures := int64(2)

	result, err := HasEnoughSignatures(members, percentage, precision, signatures)

	assert.Nil(t, err)
	assert.True(t, result)
}
