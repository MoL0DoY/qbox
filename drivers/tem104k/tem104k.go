package tem104k

import (
	"bytes"
	"encoding/hex"
	"qBox/drivers"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"time"
)

/*
Драйвер согласно протоколу ТЭМ-104K
*/
type Tem104K struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
}

// Реализация интерфейса IDeviceDriver::Init
func (tem *Tem104K) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	tem.logger = logger
	tem.network = network
	tem.counterNumber = counterNumber

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Идентификация устройства № %d", tem.counterNumber)
	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x00, 0x00, 0x00}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkDevice
	_, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}

	tem.logger.Info("Получение версии ПО устройства")
	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x00, 0x01, 0x00}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkSoftVersion
	_, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}

	tem.logger.Info("Чтение памяти EEPROM 512 байт")
	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x00, 0x00, 0x08}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	response, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}
	tem.data.AddNewSystem(0)
	tem.data.Systems[0].Status = true

	//
	//// Ед. измерения tem.data.UnitQ
	//// В ТЭМ-10statX показываются ГКал и цифра, эта же цифра получается и здесь, но по протоколу она указана как МВт.
	//tem.data.UnitQ = models.Gcal // Этот случай перепроверен на ОДК, действительно с прибора приходят сразу ГКал
	//
	tem.data.Serial = string(response[6:13])
	logger.Debug("Байты заводского номера (%s) - %s.", response[6:13], tem.data.Serial)

	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (tem *Tem104K) Read() (*models.DataDevice, error) {

	tem.data.TimeRequest = time.Now()

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Получение даты времени на теплосчётчике")
	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x0F, 0x02, 0x02, 0x00, 0x07}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	year := 2000 + drivers.DecodeBcd([]byte{response[12]})
	month := time.Month(drivers.DecodeBcd([]byte{response[11]}))
	day := drivers.DecodeBcd([]byte{response[10]})
	hour := drivers.DecodeBcd([]byte{response[8]})
	min := drivers.DecodeBcd([]byte{response[7]})
	sek := drivers.DecodeBcd([]byte{response[6]})
	tem.data.Time = time.Date(year, month, day, hour, min, sek, 0, time.Local)

	tem.logger.Info("Чтение интеграторов")

	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x01, 0x40, 0x30}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].V1 = float64(float32(tem.readLongFrom(response, 0x06)) + tem.readFloatFrom(response, 0x06+0x04))
	tem.data.Systems[0].M1 = float64(float32(tem.readLongFrom(response, 0x06+0x08)) + tem.readFloatFrom(response, 0x06+0x08+0x04))
	tem.data.Systems[0].SigmaQ = float64(float32(tem.readLongFrom(response, 0x06+0x08+0x08)) + tem.readFloatFrom(response, 0x06+0x08+0x08+0x04))
	tem.data.UnitQ = models.Gcal

	tem.data.TimeRunCommon = tem.readLongFrom(response, 0x06+0x08+0x08+0x08+0x10)
	tem.data.Systems[0].TimeRunSys = tem.data.TimeRunCommon
	tem.data.TimeOn = tem.data.TimeRunCommon + tem.readLongFrom(response, 0x06+0x08+0x08+0x08+0x10+0x04)

	tem.logger.Info("Чтение значений текущих температур")

	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03, 0x01, 0x08, 0x08}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].T1 = tem.readFloatFrom(response, 0x06)
	tem.data.Systems[0].T2 = tem.readFloatFrom(response, 0x06+0x04)

	tem.logger.Info("Чтение значений текущих расходов")

	command = []byte{0x55, tem.counterNumber, drivers.ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03, 0x00, 0xB4, 0x04}
	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].GV1 = tem.readFloatFrom(response, 0x06)

	return &tem.data, nil
}

/**
* Проверка контрольной суммы
 */
func (*Tem104K) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return ^(sum & 0xFF)
}

func (tem *Tem104K) checkFrame(response []byte) bool {
	if len(response) < 6 {
		tem.logger.Info("Получено меньше 6 байт")
		return false
	}

	if response[0] != 0xAA || ((^response[1])&0xFF) != response[2] {
		tem.logger.Info("Заголовок ответа не верный: %X", response[0:5])
		return false
	}

	if len(response) < 6+int(response[5])+1 {
		tem.logger.Info("Размер полученных данных меньше ожидаемого: %X", 6+int(response[5])+1)
		return false
	}

	checkSum := response[len(response)-1]
	calculatedCheckSum := tem.calculateCheckSum(response[:len(response)-1])
	if calculatedCheckSum != checkSum {
		tem.logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		tem.logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		tem.logger.Debug("Получена контрольная сумма- %X", checkSum)
		return false
	}
	tem.logger.Info("Проверка переданных данных: УСПЕШНО.")
	return true
}

func (*Tem104K) readLongFrom(response []byte, cursor int) uint32 {
	long := [4]byte{
		response[cursor],
		response[cursor+1],
		response[cursor+2],
		response[cursor+3]}

	return drivers.ToLong(long)
}

func (*Tem104K) readFloatFrom(response []byte, cursor int) float32 {
	float := [4]byte{
		response[cursor],
		response[cursor+1],
		response[cursor+2],
		response[cursor+3]}

	return drivers.ToFloat(float)
}

func (tem *Tem104K) checkDevice(response []byte) bool {
	if len(response) < 0x0D {
		tem.logger.Info("Ответ не той длинны.")
		return false
	}

	expect, _ := hex.DecodeString("D2C5CC2D313031")
	if bytes.Equal(response[6:13], expect) {
		tem.logger.Info("Определен тип устройства ТЭМ-101, что корректно согласно протоколу ТЭМ-104K")
		return true
	} else {
		tem.logger.Info("Ответ устройства не совпадает с ожиданием d2 c5 cc 2d 31 30 31 (ТЭМ-101). %X", response[6:13])
		return false
	}
}

func (tem *Tem104K) checkSoftVersion(response []byte) bool {
	if len(response) < 0x0D {
		tem.logger.Info("Ответ не той длинны.")
		return false
	}

	expect, _ := hex.DecodeString("76322E")
	if bytes.Equal(response[6:9], expect) {
		tem.logger.Info("Версия ПО v2.")
		return true
	} else {
		tem.logger.Info("Ответ устройства не совпадает с ожиданием 76322E (v2.). %X", response[6:9])
		return false
	}
}
