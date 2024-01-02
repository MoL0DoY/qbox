package skm2

import (
	"encoding/hex"
	"qBox/drivers/skm2/data"
	"qBox/drivers/skm2/systems"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"time"
)

type SKM struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
	checks        data.Checks
}

/**
counterNumber для СКМ:
254 (0xFE) - воспринимается всеми теплосчетчиками, вне зависимости от их адресов.
0 адрес принадлежит несконфигурированным теплосчётчикам
1-250 - принадлежат ведомым теплосчётчикам.
*/
func (skm *SKM) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	skm.logger = logger
	skm.network = network
	skm.counterNumber = counterNumber
	skm.checks = data.Checks{Logger: logger}

	// Согласно переписке с производителем СКМ-2 счётчиков.
	// ПО верхнего уровня для преобразования использует коэффициент:
	// для МВт в ГКал 1.163(разделить), МВт в ГДж - 3.6(умножение)
	skm.data.CoefficientMWh = 1 / 1.163 // 0.85984522785
	skm.data.CoefficientKWh = 1 / 1.163 / 1000
	skm.data.CoefficientGJ = 1 / (3.6 / skm.data.CoefficientMWh) //0,238845896625

	skm.logger.Info("Запрос на инициализацию прибора, № %d", skm.counterNumber)
	request := net.PrepareRequest([]byte{
		0x10,
		0x40, skm.counterNumber,
		skm.checks.CalculateCheckSum([]byte{0x40, skm.counterNumber}),
		0x16})
	request.ControlFunction = skm.checks.CheckSimpleFrame
	_, err := skm.network.RunIO(request)
	return err
}

/**
Чтение текущих данных для СКМ-2 согласно протоколу M-bus EN 60870-5
*/
func (skm *SKM) Read() (*models.DataDevice, error) {

	skm.logger.Info("Запрос на чтение текущих данных")
	request := net.PrepareRequest([]byte{
		0x68, 0x04, 0x04, 0x68,
		0x53, skm.counterNumber,
		0x50, 0x10,
		skm.checks.CalculateCheckSum([]byte{0x53, skm.counterNumber, 0x50, 0x10}), 0x16})
	request.ControlFunction = skm.checks.CheckSimpleFrame
	_, err := skm.network.RunIO(request)
	if err != nil {
		return &skm.data, err
	}

	skm.data.TimeRequest = time.Now()

	skm.logger.Info("Запрос на просмотр ответа текущих данных")
	request = net.PrepareRequest([]byte{0x10, 0x5B, skm.counterNumber,
		skm.checks.CalculateCheckSum([]byte{0x5B, skm.counterNumber}), 0x16})
	request.ControlFunction = skm.checks.CheckLongFrame
	response, err := skm.network.RunIO(request)
	for err != nil {
		return &skm.data, err
	}

	skm.data.Serial = hex.EncodeToString([]byte{response[10], response[9], response[8], response[7]})

	c := systems.Common{DataDevice: &skm.data}
	c.PopulateFromBytes(response[19:])

	skm.data.AddNewSystem(1)
	fS := systems.FirstSystem{System: &skm.data.Systems[0]}
	fS.PopulateFromBytes(response[19:])

	sS := systems.SecondSystem{System: &skm.data.Systems[1]}
	sS.PopulateFromBytes(response[19:])

	return &skm.data, nil
}