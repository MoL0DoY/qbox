package drivers

import (
	"qBox/models"
	"qBox/services/log"
	"qBox/services/net"
	"strconv"
	"time"
)

// Шаблонный код для драйверов
// После создания нового драйвера, нужно добавить его в карту драйверов services/config/config.go
type TEM05OLD struct {
	data    models.DataDevice
	network *net.Network
	logger  *log.LoggerService
}

// Реализация интерфейса IDeviceDriver::Init
func (tem05 *TEM05OLD) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	tem05.logger = logger
	tem05.network = network
	tem05.data.UnitQ = models.MWh // в других не измеряет
	tem05.data.AddNewSystem(0)
	tem05.data.Systems[0].Status = true
	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (tem05 *TEM05OLD) Read() (*models.DataDevice, error) {

	/**
	Иногда для инициализации ТЭМ-05М в официального ПО надо нажать кнопку "Интерф. адаптер", эта кнопка отправляет 4 байта.
	Тут поступаем так же.
	*/
	tem05.logger.Info("Чтение текущих")
	request := net.PrepareRequest([]byte{0x33, 0x81, 0x7e, 0x32})
	request.SecondsReadTimeout = 8
	response, err := tem05.network.RunIO(request)
	for err != nil {
		return &tem05.data, err
	}

	tem05.data.TimeRequest = time.Now()
	tem05.logger.Debug("Начата расшифровка пакета")
	tem05.populate(response)
	return &tem05.data, nil
}

func (tem05 *TEM05OLD) populate(response []byte) {

	//=======================================ВРЕМЯ======================================================================
	year := 2000 + int(ByteFromBDC(response[9]))
	month := time.Month(int(ByteFromBDC(response[8])))
	day := int(ByteFromBDC(response[7]))
	hour := int(ByteFromBDC(response[5])<<8 + ByteFromBDC(response[4]))
	min := int(ByteFromBDC(response[3])<<8 + ByteFromBDC(response[2]))
	tem05.data.Time = time.Date(year, month, day, hour, min, 0, 0, time.Local)
	//=====================================НОМЕР ПРИБОРА================================================================
	tem05.data.Serial = strconv.Itoa(int(toWord([2]byte{response[15], response[14]})))
	tem05.logger.Debug("Серийный номер прибора: " + tem05.data.Serial) //OK
	tem05.logger.Debug("Версия ПО:" + strconv.Itoa(int(response[16]))) //OK
	tem05.logger.Debug("Схема установки:" + strconv.Itoa(int(response[17])))
	//==========================НАРАБОТКА===============================================================================
	tem05.logger.Debug("Минуты наработки:" + strconv.Itoa(int(response[24])))
	hoursX := ToLong([4]byte{0, response[27], response[26], response[25]})
	tem05.logger.Debug("Часы наработки:%d", hoursX)
	tem05.data.TimeOn = uint32(hoursX*3600 + uint32(response[24])*60)
	tem05.data.TimeRunCommon = tem05.data.TimeOn // прибор не считает время наработки без ошибок.

	/**
	Коды диаметров и расходов по каналам влияют на количество знаков после запятой для значений G, V, Q значений.
	Таблица взята из ПРИЛОЖЕНИЯ 3 . Страница 38, паспорта теплостёчика
	За основу взят коэффициент V. Коэффициент для Q соответственно будет V коэффициент * 10. Для G = Vк * 100
	Экспериментально выведено, что в таблице код диаметра начинается с 0, что соответствует первой строке таблице.

	17.12.2018 Обнаружены новые сведения для данных размерностей. Появился счётчик, у которого в настройках прописаны
	диаметры 32мм c тремя разными Gmax, что соответствует кодам 0x15 0x16 и 0x17. Эти коды добавлены в текущую таблицу.
	Также важно, что у этого же счётчика по паспорту отсутствуют коды 0, 1, 2, а код 3 соответствует совершенно другим
	Gmax и размерности. С кодом 3 могут возникнуть проблемы в будущем, но вероятность очень низкая, так как в нашей
	работе трубы с таким маленьким диаметром 10 и 15мм встречаются крайне редко. Размерность для кода 3
	изменена с 1000 на 100.
	**/
	dimension := []float32{
		1000.0, 1000.0, 100.0,
		100.0, // код 3 в одном протоколе - D=15 Gmax=1,2500 dimension= 100, а в другом - D=15 Gmax=0.625 dimension= 1000
		100.0, 100.0,
		100.0, 100.0, 10.0,
		10.0, 10.0, 10.0,
		10.0, 10.0, 1.0,
		10.0, 1.0, 1.0,
		1.0, 1.0, 1.0,
		100, 10, 10}

	tem05.logger.Debug("Код диаметра и расхода 1 канала (hex):%2X", response[18])
	factor1 := dimension[int(response[18])]
	tem05.logger.Debug("Код диаметра и расхода 2 канала (hex):%2X", response[19])
	factor2 := dimension[int(response[19])]

	//============================ 1-й канал ===========================================================================
	tem05.data.Systems[0].GV1 = float32(ToLong([4]byte{0, response[34], response[33], response[32]})) / (factor1 * 100)
	tem05.logger.Debug("Расход по 1 каналу м3/ч (as float32): %4f", tem05.data.Systems[0].GV1)

	tem05.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{0, response[40], response[39], response[38]})) / (factor1 * 10))
	tem05.logger.Debug("Энергия по 1 каналу, МВт*ч :%f", tem05.data.Systems[0].Q1)

	tem05.data.Systems[0].V1 = float64(float32(ToLong([4]byte{0, response[43], response[42], response[41]})) / factor1)
	tem05.logger.Debug("Объем по 1 каналу, м3 :%f", tem05.data.Systems[0].V1)

	tem05.data.Systems[0].M1 = float64(float32(ToLong([4]byte{0, response[46], response[45], response[44]})) / factor1)
	tem05.logger.Debug("Масса по 1 каналу, т  :%f", tem05.data.Systems[0].M1)
	//============================ 2-й канал ===========================================================================
	tem05.data.Systems[0].GV2 = float32(ToLong([4]byte{0, response[49], response[48], response[47]})) / (factor2 * 100)
	tem05.logger.Debug("Расход по 2 каналу м3/ч (as float32): %4f", tem05.data.Systems[0].GV2)

	tem05.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{0, response[55], response[54], response[53]})) / (factor2 * 10))
	tem05.logger.Debug("Энергия по 2 каналу, МВт*ч :%f", tem05.data.Systems[0].Q2)

	tem05.data.Systems[0].V2 = float64(float32(ToLong([4]byte{0, response[58], response[57], response[56]})) / factor2)
	tem05.logger.Debug("Объем по 2 каналу, м3 :%f", tem05.data.Systems[0].V2)

	tem05.data.Systems[0].M2 = float64(float32(ToLong([4]byte{0, response[61], response[60], response[59]})) / factor2)
	tem05.logger.Debug("Масса по 2 каналу, т  :%f", tem05.data.Systems[0].M2)

	//======================Температуры всякие==========================================================================
	tem05.data.Systems[0].T1 = float32(toWord([2]byte{response[207], response[206]})) / 100
	tem05.logger.Debug("Температура на подаче, С:%f", tem05.data.Systems[0].T1)
	tem05.data.Systems[0].T2 = float32(toWord([2]byte{response[209], response[208]})) / 100
	tem05.logger.Debug("Температура на обратке, С:%f", tem05.data.Systems[0].T2)

	tem05.data.Systems[0].T3 = float32(toWord([2]byte{response[211], response[210]})) / 100
	tem05.logger.Debug("Температура холодной воды, С:%f", tem05.data.Systems[0].T3)

	tem05.logger.Debug("Программируемая температура, С:%f", float32(response[21])/10)
	//=========================Ошибки===================================================================================
	tem05.logger.Debug("Ошибки (байт 62 - общее количество: %d", response[62])
	tem05.logger.Debug("Ошибка 1 (203-202): %d", toWord([2]byte{response[203], response[202]}))
	tem05.logger.Debug("Ошибка 2 (205-204): %d", toWord([2]byte{response[205], response[204]}))
	//==================================================================================================================
}

func (tem05 TEM05OLD) checkResponse(response []byte) bool {
	if len(response) <= 345 {
		tem05.logger.Debug("Размер данных не соответствует ожидаемому. Получено меньше 344(345) байт")
		return false
	}
	return true
}
