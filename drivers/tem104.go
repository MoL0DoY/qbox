package drivers

import (
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

/*
Драйвер согласно протоколу ТЭМ-104

Примечание:
Память 2К таймера содержит структуру:
SysInt. Эта структура занимает 0xFF памяти. Начинается с 0200h адраса 2К памяти. В структуре содержатся интеграторы по
всем системам и каналам сразу.
SysCon. Эта структура занимает  0x19 памяти. Начинается с 0600h адреса 2к памяти. В структуре содержаться данные по
конфигурации одной системы и её каналам.

Оперативная память содержит структуру:
SysPar. Эта структура занимает  0x92 памяти. Начинается с 2200h адреса оперативной памяти. В структуре содержаться ряд
текущих данных по одной системе и её каналам.
*/
type Tem104 struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
	systemCount   int // количество активных систем
}

// Реализация интерфейса IDeviceDriver::Init
func (tem *Tem104) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	tem.logger = logger
	tem.network = network
	tem.counterNumber = counterNumber

	var command []byte
	var response []byte
	var err error

	// Читаем Память таймера 2К байт, от 0000 до 0080(0x7C + 0x04)
	// Определяем число активных систем, заводской номер.
	// 0000 - Число систем, 1 байт
	// 007С - Заводской номер прибора, 4 байта
	tem.logger.Info("Запрос на инициализацию прибора, № %d", tem.counterNumber)
	tem.logger.Info("Читаем 2K память")
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x00, 0x00, 0x7C + 0x04}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = func(response []byte) bool {
		// ответ должен содержать заголовок в 6 байт, данные размером как запрашивали и контрольную сумму.
		if len(response) != int(0x06+0x80+0x01) {
			tem.logger.Info("Ответ от прибора некорректный")
			return false
		}
		return true
	}
	response, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}

	tem.systemCount = int(response[6])
	tem.data.AddNewSystem(tem.systemCount)
	for i := 0; i <= tem.systemCount-1; i++ {
		tem.data.Systems[i].Status = true
	}
	logger.Debug("Активировано систем - %d", tem.systemCount)

	// Ед. измерения tem.data.UnitQ
	// В ТЭМ-10statX показываются ГКал и цифра, эта же цифра получается и здесь, но по протоколу она указана как МВт.
	tem.data.UnitQ = models.Gcal // Этот случай перепроверен на ОДК, действительно с прибора приходят сразу ГКал

	tem.data.Serial = strconv.FormatUint(uint64(tem.readLongFrom(response, 6+0x7C)), 10)
	logger.Debug("Байты заводского номера (%s) - %X", tem.data.Serial, response[6+0x7C:6+0x7C+4])

	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (tem *Tem104) Read() (*models.DataDevice, error) {

	tem.data.TimeRequest = time.Now()

	var command []byte
	var response []byte
	var err error

	tem.logger.Info("Получение даты времени на теплосчётчике")
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x02, 0x02, 0x10, 0x10}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = func(response []byte) bool {
		if len(response) < 15 {
			tem.logger.Info("Полученные данные меньше 15 байт")
			return false
		}
		return true
	}
	response, err = tem.network.RunIO(request)
	for err != nil {
		return &tem.data, err
	}

	year := 2000 + DecodeBcd([]byte{response[11]})
	month := time.Month(DecodeBcd([]byte{response[10]}))
	day := DecodeBcd([]byte{response[9]})
	hour := DecodeBcd([]byte{response[8]})
	min := DecodeBcd([]byte{response[7]})
	sek := DecodeBcd([]byte{response[6]})
	tem.data.Time = time.Date(year, month, day, hour, min, sek, 0, time.Local)

	tem.logger.Info("Чтение оперативной памяти")

	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03}

	for i, system := range tem.data.Systems {
		if system.Status == false {
			continue
		}
		// Текущие данные в оперативной памяти начинаются с 2200h = 8704(dec),
		// по 92h = 146(dec) байт на стркутуру по одной системе
		startBytes := intToBigEndian(uint16(8704 + 146*i))

		command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0C, 0x01, 0x03}

		command = append(command, startBytes...) // Добавляем адрес оперативной памяти

		command = append(command, 0x60) // Длина считываемого блока. Данные следующие за мощностью(0x60) не нужны.

		request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
		request.ControlFunction = tem.checkFrame
		response, err = tem.network.RunIO(request)
		for err != nil {
			return &tem.data, err
		}

		// TODO Есть ещё T4/P4/GV3/GM3/GV4/GM4 - необходимо эксперементировать с теплосчётчиком, меняяя его настройки.
		tem.data.Systems[i].T1 = tem.readFloatFrom(response, 0x06+0x00)
		tem.data.Systems[i].T2 = tem.readFloatFrom(response, 0x06+0x04)
		tem.data.Systems[i].T3 = tem.readFloatFrom(response, 0x06+0x08)

		tem.data.Systems[i].P1 = tem.readFloatFrom(response, 0x06+0x10)
		tem.data.Systems[i].P2 = tem.readFloatFrom(response, 0x06+0x14)
		tem.data.Systems[i].P3 = tem.readFloatFrom(response, 0x06+0x18)

		tem.data.Systems[i].GV1 = tem.readFloatFrom(response, 0x06+0x40)
		tem.data.Systems[i].GV2 = tem.readFloatFrom(response, 0x06+0x44)

		tem.data.Systems[i].GM1 = tem.readFloatFrom(response, 0x06+0x50)
		tem.data.Systems[i].GM2 = tem.readFloatFrom(response, 0x06+0x54)
	}

	tem.logger.Info("Читаем 2K память")

	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x02, 0x00, 0xFF}

	var memoryResponse2K []byte
	memoryResponse2K = response

	request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.SecondsReadTimeout = 5
	request.ControlFunction = tem.checkFrame
	response, err = tem.network.RunIO(request)

	if err == nil {
		memoryResponse2K = response
	} else {
		/**
		На одном объекте при чтении 2К памяти происходил "затуп" чтения на половине ответа.
		Оборудование iRZ ATM2-485 + ТЭМ-104/2 с одной активизированной системой.
		Выяснилось, что на запрос []byte{... 0x0F, 0x01, 0x03, 0x02, 0x00, 0xFF} он не может корректно ответить,
		так как FF подразумевает чтение сразу всей памяти с 0200 по 02FF и отправку большого ответа (загловок 6 байт, 256 данных и контрольная сумма).
		Что-то где-то затыкалось и в ответ приходило около 10 байт(в среднем).
		Переписав команду на два запроса: 0xFF / 2 = 0x7F - удалось получить корректный ответ.
		*/
		command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x02, 0x00, 0x7F}

		request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
		request.SecondsReadTimeout = 5
		request.ControlFunction = tem.checkFrame
		response, err = tem.network.RunIO(request)
		for err != nil {
			return &tem.data, err
		}
		memoryResponse2K = response

		command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, 0x02, 0x7F, 0x7F}
		request = net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
		request.SecondsReadTimeout = 5
		request.ControlFunction = tem.checkFrame
		response, err = tem.network.RunIO(request)
		for err != nil {
			return &tem.data, err
		}
		memoryResponse2K = append(memoryResponse2K, response[6:]...) // отбрасываем заголовок второго ответа в 6 байт.
	}

	for i, system := range tem.data.Systems {
		if system.Status == false {
			continue
		}
		tem.data.Systems[i].SigmaQ = float64(float32(tem.readLongFrom(memoryResponse2K, 0x06+0x58+0x04*i)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x28+0x04*i))
	}

	// есть V1,V2, V3 и V4 по каналам . В какие системы их помещать непонятно. Для первой системы, чаще всего V1 и V2 имеется
	tem.data.Systems[0].V1 = float64(float32(tem.readLongFrom(memoryResponse2K, 0x06+0x38)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x08))
	tem.data.Systems[0].V2 = float64(float32(tem.readLongFrom(memoryResponse2K, 0x06+0x3C)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x0C))
	//V3 = float32(tem.readLongFrom(memoryResponse2K, 0x06+0x40)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x16)
	//V4 = float32(tem.readLongFrom(memoryResponse2K, 0x06+0x44)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x1A)

	// тоже самое, что и с V
	tem.data.Systems[0].M1 = float64(float32(tem.readLongFrom(memoryResponse2K, 0x06+0x48)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x18))
	tem.data.Systems[0].M2 = float64(float32(tem.readLongFrom(memoryResponse2K, 0x06+0x4C)) + tem.readFloatFrom(memoryResponse2K, 0x06+0x1C))
	//M3 = float32(tem.readLongFrom(memoryResponse2K, 0x56)) + tem.readFloatFrom(memoryResponse2K, 0x26)
	//M4 = float32(tem.readLongFrom(memoryResponse2K, 0x5A)) + tem.readFloatFrom(memoryResponse2K, 0x2A)

	tem.data.TimeOn = tem.readLongFrom(memoryResponse2K, 0x6E)

	for i, system := range tem.data.Systems {
		if system.Status == false {
			continue
		}
		tem.data.Systems[i].TimeRunSys = tem.readLongFrom(memoryResponse2K, 0x72+i*0x4) // c 0x72 по 0x7E по 4 байта на систему
	}

	// Имеются давления по всем системам (p1 - p3), предположительно сюда попают из оперативной памяти
	// В оперативной памяти заведено P1,P2,P3,P4 по каналам. А тут только p1,p2,p3.
	// Есть предположение, что они согласно настройкам ложатся сюда как подача, обратка, техническая.
	//tem.data.Systems[0].P1 = float32(toLong([4]byte{0x00, 0x00, 0x00, memoryResponse2K[0xE6]})) / 100
	//tem.data.Systems[0].P2 = float32(toLong([4]byte{0x00, 0x00, 0x00, memoryResponse2K[0xE7]})) / 100
	//tem.data.Systems[0].P3 = float32(toLong([4]byte{0x00, 0x00, 0x00, memoryResponse2K[0xE8]})) / 100

	// Имеются температуры по всем системам(t1-t3), предположительно сюда попают из оперативной памяти
	// В оперативной памяти заведено T1,T2,T3,T4 по каналам . А тут только T1,T2,T3.
	// Есть предположение, что они согласно настройкам ложатся сюда как подача, обратка, техническая.
	//tem.data.Systems[0].T1 = float32(toLong([4]byte{0x00, 0x00, memoryResponse2K[0xCE], memoryResponse2K[0xCF]})) / 100
	//tem.data.Systems[0].T2 = float32(toLong([4]byte{0x00, 0x00, memoryResponse2K[0xD0], memoryResponse2K[0xD1]})) / 100
	//tem.data.Systems[0].T3 = float32(toLong([4]byte{0x00, 0x00, memoryResponse2K[0xD2], memoryResponse2K[0xD3]})) / 100

	return &tem.data, nil
}

/**
* Проверка контрольной суммы
 */
func (*Tem104) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return ^(sum & 0xFF)
}

func (tem *Tem104) checkFrame(response []byte) bool {
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

func (*Tem104) readLongFrom(response []byte, cursor int) uint32 {
	long := [4]byte{
		response[cursor],
		response[cursor+1],
		response[cursor+2],
		response[cursor+3]}

	return ToLong(long)
}

func (*Tem104) readFloatFrom(response []byte, cursor int) float32 {
	float := [4]byte{
		response[cursor],
		response[cursor+1],
		response[cursor+2],
		response[cursor+3]}

	return ToFloat(float)
}
