package data

import (
	"bytes"
)

type Grabber struct {
	/**
	* Data records (sometimes called variable data blocks)
	* contain the measured data. Each data record is made up of a data
	* information block (DIB), a value information block (VIB) and a value.
	* Similar to OBIS codes DIBs and VIBs code information such as
	* the meaning of a value.
	 */
	Datum []byte
}

/**
* block - байты DIB и VIB
* lengthValueBlock - количество байт, которое занимает value
 */
func (grabber *Grabber) GrabValueBytes(block []byte, lengthValueBlock int) []byte {
	index := bytes.Index(grabber.Datum, block)

	if index == -1 {
		return []byte{}
	}

	startIndex := index + len(block)
	finishIndex := startIndex + lengthValueBlock
	if len(grabber.Datum) < finishIndex {
		return []byte{}
	}

	return grabber.Datum[startIndex:finishIndex]
}