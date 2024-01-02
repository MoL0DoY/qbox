package models

import (
	logService "qBox/services/log"
	netService "qBox/services/net"
)

type IDeviceDriver interface {
	/*
		Инициализация драйвера.
		Ядро программы при инициализации драйвера вызовет этот метод и передаст следующие параметры:

		counterNumber - номер теплосчётчика. Может принимать 255 значений, 0x00 - 0xFF, в зависимости от модели счётчика.
		network - сервис netService.Network
		logger - сервис logService.LoggerService

		Эти параметры следует сохранить - возможно, они понадобятся для реализации метода Read()
	*/
	Init(counterNumber byte, network *netService.Network, logger *logService.LoggerService) error

	/**
	Чтение текущих данных теплосчётчика
	*/
	Read() (*DataDevice, error)
}
