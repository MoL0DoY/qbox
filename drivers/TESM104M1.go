package drivers

import (
	"errors"
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

type TESMART01 struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
	systemCount   int // количество активных систем
}

// функция чтения 2К памяти

func (tem *TESMART01) read2K(hi byte, lo byte, theSize byte) ([]byte, error) {
	var command []byte
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x0F, 0x01, 0x03, hi, lo, theSize}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	return tem.network.RunIO(request)
}

// Реализация интерфейса IDeviceDriver::Init
// инициализация прибора
func (tem *TESMART01) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {

	var command []byte
	var response []byte
	var err error

	tem.data.UnitQ = models.MWh // единицы Q вроде всегда одни
	tem.logger = logger
	tem.network = network
	tem.counterNumber = counterNumber

	tem.logger.Info("Старт драйвера ТЭСМАРТ.01")

	tem.logger.Info("Идентификация прибора")
	// надо послать 0х55, сет№, !сет№, 0x0F, 0x01, к-во байт 0x03, старт.ст.0x00,  старт.мл.0x00, размер прин.бл.0x1C,
	// Получено 14 байт: AA01FE000007 54534D2D313034 99
	command = []byte{0x55, tem.counterNumber, ToNotByte(tem.counterNumber), 0x00, 0x00, 0x00}
	request := net.PrepareRequest(append(command, tem.calculateCheckSum(command)))
	request.ControlFunction = tem.checkFrame
	request.SecondsReadTimeout = 5
	response, err = tem.network.RunIO(request)
	for err != nil {
		return err
	}
	if string(response[6:13]) != "TSM-104" {
		tem.logger.Debug("Получено: %s", string(response[6:13]))
		return errors.New("ответ от прибора не корректный")
	}
	logger.Debug("Получено: %s", string(response[6:13])) // наименование прибора

	// запрос на получение к-ва систем и конфигурации
	response, err = tem.read2K(0x00, 0x00, 0x07)
	for err != nil {
		return err
	}

	// 00 получаем число систем, тип систем 6-char,
	//расходомеры 6-char, ТСП 6-char, датчики Р 6-char, активные ППР 1-char, активные ТСП 1-char, активные Р 1-char ...
	// (6+208+1) проверка ответа, ответ должен содержать заголовок и данные размером как запрашивали и контрольную сумму.
	//AA01FE0F01D001000005050000010408080000030C10200000030C08200000010300FFFF00011205140142B4000042A00000428C000042700000424800004220000041F000003F66666600000000400000004040000040400000404000004040000046C7BA0046C7D20046C7AA0046C7EA0046C7B80046C7B6003F3333333ECCCCCD3F6666663F6666663F6666663F6666663F6666663F66666602020202020200000000000000000000000000000000FFFFFFFFFFFFFFFF3FCCCCCD3FCCCCCD3FCCCCCD3FCCCCCD3FCCCCCD3FCCCCCDFFFFFFFFFFFFB8
	// это проверяется в checkFrame
	// первые 6 байт ответа - заголовок АА, 01-адрес, FE-!адрес, 0F-группа команд, 01-идентификатор команды, число посылаемых байт
	tem.systemCount = int(response[6]) // количество активных систем (не более 6)
	tem.logger.Debug("Активировано систем - %d", tem.systemCount)

	// это потом поменяем , когда разберемся с многосистемными
	if response[6] != 0x01 {
		return errors.New("Это не односистемный прибор, воспользуйтесь другим драйвером!")
	}

	// запрос на чтение заводского номера прибора 4 байта и типа флэш памяти 28 байт
	response, err = tem.read2K(0x01, 0x52, 0x20)
	for err != nil {
		return err
	}
	// ответ (6+32+1) AA01FE0F012000 074631FF FFFFFFFF FFFFFFFF FFFFFFFF FFFFFFFFFF 1F25 FFFFFFFFFFFFFF5A23
	// №  1547421
	tem.data.Serial = strconv.FormatUint(uint64(tem.readLongFrom(response, 0x06)), 10) // надо исправить наверно
	logger.Debug("Номер вычислителя - %s данные - %X", tem.data.Serial, response[6:10])

	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (tem *TESMART01) Read() (*models.DataDevice, error) {
	tem.logger.Info("Чтение текущих данных")

	var response []byte
	var err error

	tem.data.AddNewSystem(1)
	tem.data.Systems[0].Status = true

	response, err = tem.read2K(0x02, 0x00, 0x68)
	for err != nil {
		return &tem.data, err
	}
	// AA01FE0F0168 | 425B4432 422822AF 00000000 00000000 00000000 00000000 00000000 | 00000000 00000000 00000000 00000000 00000000 00000000 | 3F333333 3ECCCCCD 00000000 0000
	// 				температура 0x200-0x233,										  давление 0x234-0x287,									  и расход 0x288-0x2CF
	tem.data.Systems[0].T1 = calculateFloatByPointer(response, 0x06+0x00)
	tem.data.Systems[0].T2 = calculateFloatByPointer(response, 0x06+0x04)
	tem.data.Systems[0].T3 = calculateFloatByPointer(response, 0x06+0x08)
	tem.data.Systems[0].P1 = calculateFloatByPointer(response, 0x06+0x34)      // 0x06+0x34
	tem.data.Systems[0].P2 = calculateFloatByPointer(response, 0x06+0x34+0x04) // 0x06+0x34+0x04
	tem.data.Systems[0].P3 = calculateFloatByPointer(response, 0x06+0x34+0x08) // 0x06+0x34+0x08

	// разбито на два запроса	от 0x02 0x88 L-0x48 (72)
	// запрос на чтение G 79 байт

	response, _ = tem.read2K(0x02, 0x88, 0x48)
	for err != nil {
		return &tem.data, err
	}

	tem.data.Systems[0].GV1 = calculateFloatByPointer(response, 0x06+0x00)      // 0x06+0x88
	tem.data.Systems[0].GV2 = calculateFloatByPointer(response, 0x06+0x04)      // 0x06+0x88+0x04
	tem.data.Systems[0].GM1 = calculateFloatByPointer(response, 0x06+0x18)      // 0x06+0xA0
	tem.data.Systems[0].GM2 = calculateFloatByPointer(response, 0x06+0x18+0x04) // 0x06+0xA0+0x04

	// запрос на чтение V и M, 96 байт (Float по 6 шт.)

	response, _ = tem.read2K(0x03, 0x00, 0x60)
	for err != nil {
		return &tem.data, err
	}
	// объем 0x300-0x317, масса 0x330-0x35F (Float по 6 шт.)
	tem.data.Systems[0].V1 = float64(float32(tem.readLongFrom(response, 0x06+0x18)) + tem.readFloatFrom(response, 0x06+0x00))
	tem.data.Systems[0].V2 = float64(float32(tem.readLongFrom(response, 0x06+0x1C)) + tem.readFloatFrom(response, 0x06+0x04))
	tem.data.Systems[0].M1 = float64(float32(tem.readLongFrom(response, 0x06+0x48)) + tem.readFloatFrom(response, 0x06+0x30))
	tem.data.Systems[0].M2 = float64(float32(tem.readLongFrom(response, 0x06+0x4C)) + tem.readFloatFrom(response, 0x06+0x34))

	// запрос на чтение Q, 56 байт
	response, _ = tem.read2K(0x03, 0x60, 0x38)
	for err != nil {
		return &tem.data, err
	}
	// энергия 0х360-0х3FF, 0х378 (L 4 байта) + 0x360 (F 4 байта)
	tem.data.Systems[0].Q1 = float64(float32(tem.readLongFrom(response, 0x06+0x18)) + tem.readFloatFrom(response, 0x06+0x00))
	// AA01FE0F0138 3F7E1E36 00000000 00000000 00000000 00000000 00000000 0000005E 00000000 00000000 00000000 00000000 00000000 00000000 00000000 9F

	// запрос на чтение таймеров, 28 байт
	// можно прочитать 128 байт с подробностями аварийного таймера
	// 0x404-0x41B время ошибки G<min системы 1, 2, 3, 4, 5, 6
	// 0x404-0x41B время ошибки G>max системы 1, 2, 3, 4, 5, 6
	// 0x404-0x41B время ошибки dT системы 1, 2, 3, 4, 5, 6
	// 0x404-0x41B время ошибки Тех.неиспр. системы 1, 2, 3, 4, 5, 6

	response, _ = tem.read2K(0x04, 0x00, 0x1C)
	for err != nil {
		return &tem.data, err
	}
	// 0x400-0x403 время общее
	// 0x404-0x41B время системы 1, 2, 3, 4, 5, 6
	tem.data.TimeOn = calculateLongByPointer(response, 0x06+0x00)
	tem.data.TimeRunCommon = calculateLongByPointer(response, 0x06+0x04)
	tem.data.Systems[0].TimeRunSys = calculateLongByPointer(response, 0x06+0x04)
	tem.data.TimeRequest = time.Now()

	// читаем время на приборе
	response, _ = tem.read2K(0x04, 0x82, 0x0C)
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

	return &tem.data, nil
}

func (tem *TESMART01) readLongFrom(response []byte, cursor int) uint32 {
	long := [4]byte{
		response[cursor],
		response[cursor+1],
		response[cursor+2],
		response[cursor+3]}

	return ToLong(long)
}

func (tem *TESMART01) readFloatFrom(response []byte, cursor int) float32 {
	float := [4]byte{
		response[cursor],
		response[cursor+1],
		response[cursor+2],
		response[cursor+3]}

	return ToFloat(float)
}

func (tem *TESMART01) checkFrame(response []byte) bool {
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

/**
* Проверка контрольной суммы
 */
func (*TESMART01) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return ^(sum & 0xFF)
}
