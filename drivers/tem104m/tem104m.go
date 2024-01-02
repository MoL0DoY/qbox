package tem104m

import (
	"qBox/models"
	"qBox/services/convert"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

/*
Драйвер согласно протоколу ТЭМ-104М от 2018-09-12 - 2018-10-09
*/
type TEM104M struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
}

// Реализация интерфейса IDeviceDriver::Init
func (tem *TEM104M) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	tem.logger = logger
	tem.network = network
	tem.counterNumber = counterNumber

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Инициализация прибора, № %d", tem.counterNumber)
	tem.logger.Info("Читаем заводской номер")
	command = []byte{0x55, tem.counterNumber, convert.ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x00, 0x00, 0x07}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.SecondsReadTimeout = 5
	request.ControlFunction = tem.checkFrame
	response, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}

	numberSystem := int(response[6+4])
	if numberSystem <= 4 && numberSystem >= 1 {
		tem.data.AddNewSystem(numberSystem - 1)
		tem.logger.Debug("Определено (%d) количество систем", numberSystem)
	} else {
		tem.logger.Debug("Определено некорректное (%X) количество систем.", numberSystem)
		tem.data.AddNewSystem(1)
	}

	tem.data.UnitQ = models.Gcal

	tem.data.Serial = strconv.FormatUint(uint64(convert.LongLittleEndianByPointer(response, 0x06)), 10)
	tem.logger.Debug("Байты заводского номера (%s) - %X", tem.data.Serial, response[6:6+4])
	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (tem *TEM104M) Read() (*models.DataDevice, error) {
	var command []byte
	var response []byte
	var err error

	tem.data.TimeRequest = time.Now()
	tem.populateDatetime()

	tem.logger.Info("Чтение оперативной памяти")

	command = []byte{0x55, tem.counterNumber, convert.ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03, 0x00, 0x00, 0x60}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}
	tem.data.Systems[0].GV1 = convert.FloatLittleEndianByPointer(response, 0x06+0x40)
	tem.data.Systems[0].GM1 = convert.FloatLittleEndianByPointer(response, 0x06+0x50)
	tem.data.Systems[0].GV2 = convert.FloatLittleEndianByPointer(response, 0x06+0x44)
	tem.data.Systems[0].GM2 = convert.FloatLittleEndianByPointer(response, 0x06+0x54)

	integratorsData := tem.integratorsData()
	tem.logger.Info("Расшифровка интеграторов %X", integratorsData)

	for i, _ := range tem.data.Systems {
		tem.logger.Info("Чтение интеграторов системы %d", i+1)
		tem.data.Systems[i].Status = true
		tem.data.Systems[i].SigmaQ = float64(float32(convert.LongLittleEndianByPointer(integratorsData, uint8(0x28+i))) + convert.FloatLittleEndianByPointer(integratorsData, uint8(0x68+i)))
		tem.data.Systems[i].V1 = float64(float32(convert.LongLittleEndianByPointer(integratorsData, 0x08)) + convert.FloatLittleEndianByPointer(integratorsData, 0x48))
		tem.data.Systems[i].V2 = float64(float32(convert.LongLittleEndianByPointer(integratorsData, 0x04+0x08)) + convert.FloatLittleEndianByPointer(integratorsData, 0x04+0x48))
		tem.data.Systems[i].M1 = float64(float32(convert.LongLittleEndianByPointer(integratorsData, 0x18)) + convert.FloatLittleEndianByPointer(integratorsData, 0x58))
		tem.data.Systems[i].M2 = float64(float32(convert.LongLittleEndianByPointer(integratorsData, 0x04+0x18)) + convert.FloatLittleEndianByPointer(integratorsData, 0x04+0x58))
		tem.data.TimeOn = convert.LongLittleEndianByPointer(integratorsData, 0x98)
		tem.data.Systems[i].TimeRunSys = convert.LongLittleEndianByPointer(integratorsData, uint8(0xA0+i))
		tem.data.Systems[i].T1 = float32(convert.ToWord([2]byte{integratorsData[285], integratorsData[284]})) / 100
		tem.data.Systems[i].T2 = float32(convert.ToWord([2]byte{integratorsData[287], integratorsData[286]})) / 100
		tem.data.Systems[i].T3 = float32(convert.ToWord([2]byte{integratorsData[289], integratorsData[288]})) / 100
		tem.data.Systems[i].P1 = float32(integratorsData[308]) / 100
		tem.data.Systems[i].P2 = float32(integratorsData[309]) / 100
	}

	return &tem.data, nil
}

func (tem *TEM104M) prepareCommand(commandBytes []byte) []byte {
	command := append([]byte{0x55, tem.counterNumber, convert.ToNotByte(tem.counterNumber)}, commandBytes...)
	return append(command, tem.calculateCheckSum(command))
}

func (tem *TEM104M) populateDatetime() {
	tem.logger.Info("Получение даты времени на теплосчётчике")
	command := []byte{0x55, tem.counterNumber, convert.ToNotByte(tem.counterNumber), 0x0F, 0x02, 0x02, 0x00, 0x06}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err := tem.network.RunIO(request)
	for err != nil {
		tem.logger.Info("Ошибка получения даты времени на теплосчётчике. " + err.Error())
		return
	}

	year := 2000 + int(response[11])
	month := time.Month(int(response[10]))
	day := int(response[9])
	hour := int(response[8])
	min := int(response[7])
	sek := int(response[6])
	tem.data.Time = time.Date(year, month, day, hour, min, sek, 0, time.Local)
}

func (tem *TEM104M) integratorsData() []byte {
	tem.logger.Info("Чтение карты накопленных значений параметров (интеграторы)")
	step := 0
	var integratorsData []byte
	for {
		// Текущие данные в оперативной памяти начинаются с 0800h = 2048(dec),
		// по 0160h(015Fh+1) = 352(dec) байт на структуру по одной системе
		startBytes := convert.IntToBigEndianBytes(uint16(2048 + 0x40*step))
		// начнётся с 0x0F, 0x01, 0x03, 0x08, 0x00, 0x40
		request := net.PrepareRequest(tem.prepareCommand(append(append([]byte{0x0F, 0x01, 0x03}, startBytes...), 0x40)))
		request.ControlFunction = tem.checkFrame
		request.SecondsReadTimeout = 5
		response, err := tem.network.RunIO(request)
		for err != nil {
			tem.logger.Info("Ошибка чтения интеграторов. " + err.Error())
			return integratorsData
		}
		integratorsData = append(integratorsData, response[6:len(response)-1]...)
		if step*0x40 < 351 { // 0x01 0x5F
			step++
		} else {
			break
		}
	}
	return integratorsData
}

/**
* Проверка контрольной суммы
 */
func (*TEM104M) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return ^(sum & 0xFF)
}

func (tem *TEM104M) checkFrame(response []byte) bool {
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

	return true
}
