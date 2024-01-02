package drivers

import (
	"encoding/hex"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"time"
)

/*
Драйвер для опроска SKU-02-B теплосчётчиков
Такой же как и sku02b.go тем отличием, что в команде на чтение данных используется байт 0x7B вместо 0x5B
*/
type SKU02B7B struct {
	number byte
	sku    SKU02B
}

func (sku *SKU02B7B) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	return sku.sku.Init(counterNumber, network, logger)
}

func (sku *SKU02B7B) Read() (*models.DataDevice, error) {
	sku.sku.logger.Info("Запрос на инициализацию прибора, № %d", sku.sku.counterNumber)
	request := net.PrepareRequest([]byte{
		0x10,
		0x40, sku.sku.counterNumber,
		sku.sku.calculateCheckSum([]byte{0x40, sku.sku.counterNumber}),
		0x16})
	request.ControlFunction = sku.sku.checkSimpleFrame
	_, err := sku.sku.network.RunIO(request)
	for err != nil {
		return &sku.sku.data, err
	}

	sku.sku.logger.Info("Запрос на чтение текущих данных")
	request = net.PrepareRequest([]byte{
		0x68, 0x04, 0x04, 0x68,
		0x53, sku.sku.counterNumber,
		0x50, 0x00,
		sku.sku.calculateCheckSum([]byte{0x53, sku.sku.counterNumber, 0x50, 0x00}), 0x16})
	request.ControlFunction = sku.sku.checkSimpleFrame
	_, err = sku.sku.network.RunIO(request)
	for err != nil {
		return &sku.sku.data, err
	}

	sku.sku.data.TimeRequest = time.Now()

	sku.sku.logger.Info("Запрос на просмотр ответа текущих данных")
	request = net.PrepareRequest([]byte{0x10, 0x7B, sku.sku.counterNumber,
		sku.sku.calculateCheckSum([]byte{0x7B, sku.sku.counterNumber}), 0x16})
	request.ControlFunction = sku.sku.checkLongFrame
	response, err := sku.sku.network.RunIO(request)
	for err != nil {
		return &sku.sku.data, err
	}

	sku.sku.data.Serial = hex.EncodeToString([]byte{response[10], response[9], response[8], response[7]})
	sku.sku.populate(response[19:])
	return &sku.sku.data, nil
}