// Copyright 2015 Netflix, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package binprot

import (
	"bufio"
	"encoding/binary"
	"io"

	"github.com/netflix/rend/client/common"
)

type BinProt struct{}

func consumeResponse(r *bufio.Reader) ([]byte, error) {
	res, err := readRes(r)
	if err != nil {
		return nil, err
	}

	apperr := statusToError(res.Status)

	// read body in regardless of the error in the header
	buf := make([]byte, res.BodyLen)
	io.ReadFull(r, buf)

	// ignore extras for now
	buf = buf[res.ExtraLen:]

	resPool.Put(res)

	if apperr != nil && srsErr(apperr) {
		return buf, apperr
	}

	return buf, err
}

func consumeResponseCheckOpaque(r *bufio.Reader, opaque int) ([]byte, error) {
	res, err := readRes(r)
	if err != nil {
		return nil, err
	}

	if res.Opaque != uint32(opaque) {
		panic("SHIT")
	}

	apperr := statusToError(res.Status)

	// read body in regardless of the error in the header
	buf := make([]byte, res.BodyLen)
	io.ReadFull(r, buf)

	// ignore extras for now
	buf = buf[res.ExtraLen:]

	resPool.Put(res)

	if apperr != nil && srsErr(apperr) {
		return buf, apperr
	}

	return buf, err
}

func consumeBatchResponse(r *bufio.Reader) ([][]byte, error) {
	opcode := uint8(Get)
	var apperr error
	ret := make([][]byte, 0)

	for opcode != Noop {
		res, err := readRes(r)
		if err != nil {
			return nil, err
		}

		opcode = res.Opcode
		apperr = statusToError(res.Status)

		// read body in regardless of the error in the header
		buf := make([]byte, res.BodyLen)
		io.ReadFull(r, buf)

		// ignore extras for now
		buf = buf[res.ExtraLen:]

		ret = append(ret, buf)
	}

	return ret, apperr
}

func (b BinProt) Set(rw *bufio.ReadWriter, key, value []byte) error {
	// set packet contains the req header, flags, and expiration
	// flags are irrelevant, and are thus zero.
	// expiration could be important, so hammer with random values from 1 sec up to 1 hour

	// Header
	bodylen := 8 + len(key) + len(value)
	writeReq(rw, Set, len(key), 8, bodylen, 0)
	// Extras
	binary.Write(rw, binary.BigEndian, uint32(0))
	binary.Write(rw, binary.BigEndian, common.Exp())
	// Body / data
	rw.Write(key)
	rw.Write(value)

	rw.Flush()

	// consume all of the response and discard
	_, err := consumeResponse(rw.Reader)
	return err
}

func (b BinProt) Add(rw *bufio.ReadWriter, key, value []byte) error {
	// add packet contains the req header, flags, and expiration
	// flags are irrelevant, and are thus zero.
	// expiration could be important, so hammer with random values from 1 sec up to 1 hour

	// Header
	bodylen := 8 + len(key) + len(value)
	writeReq(rw, Add, len(key), 8, bodylen, 0)
	// Extras
	binary.Write(rw, binary.BigEndian, uint32(0))
	binary.Write(rw, binary.BigEndian, common.Exp())
	// Body / data
	rw.Write(key)
	rw.Write(value)

	rw.Flush()

	// consume all of the response and discard
	_, err := consumeResponse(rw.Reader)
	return err
}

func (b BinProt) Replace(rw *bufio.ReadWriter, key, value []byte) error {
	// replace packet contains the req header, flags, and expiration
	// flags are irrelevant, and are thus zero.
	// expiration could be important, so hammer with random values from 1 sec up to 1 hour

	// Header
	bodylen := 8 + len(key) + len(value)
	writeReq(rw, Replace, len(key), 8, bodylen, 0)
	// Extras
	binary.Write(rw, binary.BigEndian, uint32(0))
	binary.Write(rw, binary.BigEndian, common.Exp())
	// Body / data
	rw.Write(key)
	rw.Write(value)

	rw.Flush()

	// consume all of the response and discard
	_, err := consumeResponse(rw.Reader)
	return err
}

func (b BinProt) Get(rw *bufio.ReadWriter, key []byte) ([]byte, error) {
	// Header
	writeReq(rw, Get, len(key), 0, len(key), 0)
	// Body
	rw.Write(key)

	rw.Flush()

	// consume all of the response and return
	return consumeResponse(rw.Reader)
}

func (b BinProt) GetWithOpaque(rw *bufio.ReadWriter, key []byte, opaque int) ([]byte, error) {
	// Header
	writeReq(rw, Get, len(key), 0, len(key), opaque)
	// Body
	rw.Write(key)

	rw.Flush()

	// consume all of the response and return
	return consumeResponseCheckOpaque(rw.Reader, opaque)
}

func (b BinProt) BatchGet(rw *bufio.ReadWriter, keys [][]byte) ([][]byte, error) {
	for _, key := range keys {
		// Header
		writeReq(rw, GetQ, len(key), 0, len(key), 0)
		// Body
		rw.Write(key)
	}

	writeReq(rw, Noop, 0, 0, 0, 0)

	rw.Flush()

	// consume all of the responses and return
	return consumeBatchResponse(rw.Reader)
}

func (b BinProt) GAT(rw *bufio.ReadWriter, key []byte) ([]byte, error) {
	// Header
	writeReq(rw, GAT, len(key), 4, len(key), 0)
	// Extras
	binary.Write(rw, binary.BigEndian, common.Exp())
	// Body
	rw.Write(key)

	rw.Flush()

	// consume all of the response and return
	return consumeResponse(rw.Reader)
}

func (b BinProt) Delete(rw *bufio.ReadWriter, key []byte) error {
	// Header
	writeReq(rw, Delete, len(key), 0, len(key), 0)
	// Body
	rw.Write(key)

	rw.Flush()

	// consume all of the response and discard
	_, err := consumeResponse(rw.Reader)
	return err
}

func (b BinProt) Touch(rw *bufio.ReadWriter, key []byte) error {
	// Header
	writeReq(rw, Touch, len(key), 4, len(key)+4, 0)
	// Extras
	binary.Write(rw, binary.BigEndian, common.Exp())
	// Body
	rw.Write(key)

	rw.Flush()

	// consume all of the response and discard
	_, err := consumeResponse(rw.Reader)
	return err
}
