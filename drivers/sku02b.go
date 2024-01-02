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
Всегда имеет одну систему.
Примечание: Технические(согласно протоколу): Температура 3, расходы 3 и 4 не учитываются, V3,V4,V(отрицательно) тоже.
Они нигде у нас не используются.
*/
type SKU02B struct {
	data          models.DataDevice
	network       *net.Network
	logger        *log.LoggerService
	counterNumber byte
}

/**
counterNumber для SKU-02-B:
254 (0xFE) - вопринимается всеми теплосчетчиками, внезависимости от их адресов.
0 адрес принадлежит несконфигурированным теплосчётчикам
1-250 - принадлежат ведомым теплосчётчикам.
*/
func (sku *SKU02B) Init(counterNumber byte, network *net.Network, logger *log.LoggerService) error {
	sku.logger = logger
	sku.network = network
	sku.counterNumber = counterNumber
	return nil
}

// Реализация интерфейса IDeviceDriver::Read
func (sku *SKU02B) Read() (*models.DataDevice, error) {

	sku.logger.Info("Запрос на инициализацию прибора, № %d", sku.counterNumber)
	request := net.PrepareRequest([]byte{
		0x10,
		0x40, sku.counterNumber,
		sku.calculateCheckSum([]byte{0x40, sku.counterNumber}),
		0x16})
	request.ControlFunction = sku.checkSimpleFrame
	_, err := sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.logger.Info("Запрос на чтение текущих данных")
	request = net.PrepareRequest([]byte{
		0x68, 0x04, 0x04, 0x68,
		0x53, sku.counterNumber,
		0x50, 0x00,
		sku.calculateCheckSum([]byte{0x53, sku.counterNumber, 0x50, 0x00}), 0x16})
	request.ControlFunction = sku.checkSimpleFrame
	_, err = sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.data.TimeRequest = time.Now()

	sku.logger.Info("Запрос на просмотр ответа текущих данных")
	request = net.PrepareRequest([]byte{0x10, 0x5B, sku.counterNumber,
		sku.calculateCheckSum([]byte{0x5B, sku.counterNumber}), 0x16})
	request.ControlFunction = sku.checkLongFrame
	response, err := sku.network.RunIO(request)
	for err != nil {
		return &sku.data, err
	}

	sku.data.Serial = hex.EncodeToString([]byte{response[10], response[9], response[8], response[7]})
	sku.populate(response[19:])
	return &sku.data, nil
}

func (sku *SKU02B) checkSimpleFrame(response []byte) bool {
	if len(response) == 0 {
		sku.logger.Info("Получен пустой ответ.")
		return false
	}

	if len(response) > 0 && response[0] == 0xE5 {
		return true
	} else {
		sku.logger.Info("Получен некорректный ответ. Проверка SimpleFrame не пройдена.")
		return false
	}
}

func (sku *SKU02B) checkLongFrame(response []byte) bool {
	if len(response) < 19 {
		// Структура ответа на запрос текущих данных занимает от 19 байт и больше
		sku.logger.Info("Получен некорректный ответ. Ответ содержит меньше 19 байт.")
		return false
	}

	if response[0] != response[3] || response[0] != 0x68 || response[1] != response[2] {
		sku.logger.Info(
			"Получен некорректный ответ. Заголовок(68h L L 68h) ответа не верный - %X",
			response[0:3])
		return false
	}

	L := response[1] // задаём L из заголовка

	if len(response) < int(L)+4+1+1 { //  Заголовок(68h L L 68h) + L + CheckSum + Stop(16h)
		sku.logger.Info("Получен некорректный ответ. Некорректная длинна ответа, согласно заголовку (68h L L 68h).")
		return false
	}

	checkSum := response[len(response)-2]
	calculatedCheckSum := sku.calculateCheckSum(response[4 : len(response)-2])
	if calculatedCheckSum != checkSum {
		sku.logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		sku.logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		sku.logger.Debug("Получена контрольная сумма- %X", checkSum)
		return false
	}

	return true
}

/**
Проверка контрольной суммы для SKU-02-B
Представляет собой сумму значений из bytes, урезенную до одного байта
*/
func (sku *SKU02B) calculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return sum & 0xFF
}

/**
 * datum - Data records (sometimes called variable data blocks)
 * contain the measured data. Each data record is made up of a data
 * information block (DIB), a value information block (VIB) and a value.
 * Similar to OBIS codes DIBs and VIBs code information such as
 * the meaning of a value.
 */
func (sku *SKU02B) populate(datum []byte) {
	i := 0
	sku.data.AddNewSystem(1)
	sku.data.Systems[0].Status = true
	for {
		cursor := i
		switch dib := datum[cursor]; dib {
		case 0x84:
			if datum[cursor+1] == 0x40 { // Ox84 0x40 - DIF
				cursor += 2
				switch vib := datum[cursor]; vib {
				case 0xFB: // 0.1 MWh Тепловая энергия
					if datum[cursor+1] == 0x00 { //0xFB 0x00 - VIF
						cursor++
						factor := float32(0.1)
						cursor += 4
						sku.data.UnitQ = models.MWh
						sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
							datum[cursor],
							datum[cursor-1],
							datum[cursor-2],
							datum[cursor-3]})) * factor)
						break
					}
					if datum[cursor+1] == 0x08 { //0xFB 0x08 - VIF
						cursor++
						factor := float32(0.1)
						cursor += 4
						sku.data.UnitQ = models.GJ
						sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
							datum[cursor],
							datum[cursor-1],
							datum[cursor-2],
							datum[cursor-3]})) * factor)
						break
					}
				case 0x0F:
					factor := float32(0.01)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x07:
					factor := float32(0.01)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x0E:
					factor := float32(0.001)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x06:
					factor := float32(0.001)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x05:
					factor := float32(0.0001)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x0D:
					factor := float32(0.0001)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].Q1 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x16:
					factor := float32(1)
					cursor += 4
					sku.data.Systems[0].V2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x96:
				case 0x1E: // встретилось в одном из приборов
					factor := float32(1)
					cursor += 4
					sku.data.Systems[0].M2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x15:
					factor := float32(0.1)
					cursor += 4
					sku.data.Systems[0].V2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x95:
					factor := float32(0.1)
					cursor += 4
					sku.data.Systems[0].M2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x14:
					factor := float32(0.01)
					cursor += 4
					sku.data.Systems[0].V2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x94:
					factor := float32(0.01)
					cursor += 4
					sku.data.Systems[0].M2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x13:
					factor := float32(0.001)
					cursor += 4
					sku.data.Systems[0].V2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x93:
					factor := float32(0.001)
					cursor += 4
					sku.data.Systems[0].M2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				default:
					cursor -= 2 // VIB не найден, возврат курсора
				}
			}
			if datum[cursor+1] == 0x80 && datum[cursor+2] == 0x40 { // 0x84 0x80 0x40 - DIF
				cursor += 3
				switch vib := datum[cursor]; vib {
				case 0xFB:
					if datum[cursor+1] == 0x00 { //0xFB 0x00 - VIF
						cursor += 1
						factor := float32(0.1)
						cursor += 4
						sku.data.UnitQ = models.MWh
						sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
							datum[cursor],
							datum[cursor-1],
							datum[cursor-2],
							datum[cursor-3]})) * factor)
						break
					}
					if datum[cursor+1] == 0x08 { //0xFB 0x08 - VIF
						cursor += 1
						factor := float32(0.1)
						cursor += 4
						sku.data.UnitQ = models.GJ
						sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
							datum[cursor],
							datum[cursor-1],
							datum[cursor-2],
							datum[cursor-3]})) * factor)
						break
					}
				case 0x0F:
					factor := float32(0.01)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x07:
					factor := float32(0.01)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x0E:
					factor := float32(0.001)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x06:
					factor := float32(0.001)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x05:
					factor := float32(0.0001)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				case 0x0D:
					factor := float32(0.0001)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].Q2 = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				default:
					cursor -= 3 // VIB не найден, возврат курсора
				}
			}

		case 0x04:
			cursor++
			switch vib := datum[cursor]; vib {
			case 0xFB:
				if datum[cursor+1] == 0x00 { //0xFB 0x00 - VIF
					cursor += 1
					factor := float32(0.1)
					cursor += 4
					sku.data.UnitQ = models.MWh
					sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				}
				if datum[cursor+1] == 0x08 { //0xFB 0x08 - VIF
					cursor += 1
					factor := float32(0.1)
					cursor += 4
					sku.data.UnitQ = models.GJ
					sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]})) * factor)
					break
				}
			case 0x0F:
				factor := float32(0.01)
				cursor += 4
				sku.data.UnitQ = models.GJ
				sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x07:
				factor := float32(0.01)
				cursor += 4
				sku.data.UnitQ = models.MWh
				sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x0E:
				factor := float32(0.001)
				cursor += 4
				sku.data.UnitQ = models.GJ
				sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x06:
				factor := float32(0.001)
				cursor += 4
				sku.data.UnitQ = models.MWh
				sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x05:
				factor := float32(0.0001)
				cursor += 4
				sku.data.UnitQ = models.MWh
				sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x0D:
				factor := float32(0.0001)
				cursor += 4
				sku.data.UnitQ = models.GJ
				sku.data.Systems[0].SigmaQ = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x16:
				factor := float32(1)
				cursor += 4
				sku.data.Systems[0].V1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x96:
			case 0x1E: // встретилось в одном из приборов
				factor := float32(1)
				cursor += 4
				sku.data.Systems[0].M1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x15:
				factor := float32(0.1)
				cursor += 4
				sku.data.Systems[0].V1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x95:
				factor := float32(0.1)
				cursor += 4
				sku.data.Systems[0].M1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x14:
				factor := float32(0.01)
				cursor += 4
				sku.data.Systems[0].V1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x94:
				factor := float32(0.01)
				cursor += 4
				sku.data.Systems[0].M1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x13:
				factor := float32(0.001)
				cursor += 4
				sku.data.Systems[0].V1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x93:
				factor := float32(0.001)
				cursor += 4
				sku.data.Systems[0].M1 = float64(float32(ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})) * factor)
				break
			case 0x6D:
				cursor += 4
				year := 2000 + int(datum[cursor-1]>>5) + (int(datum[cursor]&0xF0) >> 1)
				month := time.Month(int(datum[cursor] & 0x0F))
				day := int(datum[cursor-1] & 0x1F)
				hour := int(datum[cursor-2])
				min := int(datum[cursor-3])
				sku.data.Time = time.Date(year, month, day, hour, min, 0, 0, time.Local)
				break
			case 0x20:
				cursor += 4
				sku.data.TimeOn = ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})
				break
			case 0x24:
				cursor += 4
				sku.data.TimeRunCommon = ToLong([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]})
				sku.data.Systems[0].TimeRunSys = sku.data.TimeRunCommon
				break
			default:
				cursor-- // VIB не найден, возврат курсора
			}

		case 0x05:
			cursor++
			switch vib := datum[cursor]; vib {
			case 0x3E:
				factor := float32(1.0)
				cursor += 4
				sku.data.Systems[0].GV1 = ToFloat([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]}) * factor
				break
			case 0x56:
				factor := float32(1.0)
				cursor += 4
				sku.data.Systems[0].GM1 = ToFloat([4]byte{
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2],
					datum[cursor-3]}) * factor
				break
			default:
				cursor-- // VIB не найден, возврат курсора
			}

		case 0x85:
			if datum[cursor+1] == 0x40 { //0x85 0x40 - DIB
				cursor += 2
				switch vib := datum[cursor]; vib {
				case 0x3E:
					factor := float32(1.0)
					cursor += 4
					sku.data.Systems[0].GV2 = ToFloat([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]}) * factor
					break
				case 0x56:
					factor := float32(1.0)
					cursor += 4
					sku.data.Systems[0].GM2 = ToFloat([4]byte{
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2],
						datum[cursor-3]}) * factor
					break
				default:
					cursor -= 2 // VIB не найден, возврат курсора
				}
			}

		case 0x02:
			cursor++
			switch vib := datum[cursor]; vib {
			case 0x59:
				factor := float32(0.01)
				cursor += 2
				sku.data.Systems[0].T1 = float32(toWord([2]byte{datum[cursor], datum[cursor-1]})) * factor
				break
			case 0x5D:
				factor := float32(0.01)
				cursor += 2
				sku.data.Systems[0].T2 = float32(toWord([2]byte{datum[cursor], datum[cursor-1]})) * factor
				break
			default:
				cursor-- // VIB не найден, возврат курсора
			}

		case 0x82:
			if datum[cursor+1] == 0x40 { //0x82 0x40 - DIB
				cursor += 2
				switch vib := datum[cursor]; vib {
				case 0x65:
					factor := float32(0.01)
					cursor += 2
					sku.data.Systems[0].T3 = float32(toWord([2]byte{datum[cursor], datum[cursor-1]})) * factor
					break
				case 0x59: // температура 3
					break
				default:
					cursor -= 2 // VIB не найден, возврат курсора
				}
			}

		case 0x03:
			cursor++
			switch vib := datum[cursor]; vib {
			case 0x68:
				factor := float32(0.0001)
				cursor += 3
				sku.data.Systems[0].P1 = float32(ToLong([4]byte{
					0x00,
					datum[cursor],
					datum[cursor-1],
					datum[cursor-2]})) * factor
				break
			default:
				cursor-- // VIB не найден, возврат курсора
			}

		case 0x83:
			if datum[cursor+1] == 0x40 { //0x83 0x40 - DIB
				cursor += 2
				switch vib := datum[cursor]; vib {
				case 0x68:
					factor := float32(0.0001)
					cursor += 3
					sku.data.Systems[0].P2 = float32(ToLong([4]byte{
						0x00,
						datum[cursor],
						datum[cursor-1],
						datum[cursor-2]})) * factor
					break
				default:
					cursor -= 2 // VIB не найден, возврат курсора
				}
			}
		}

		if i == cursor {
			cursor++
		}

		i = cursor

		if i >= len(datum) {
			//fmt.Println("Выполенно: 100%")
			break
		}

		if len(datum) != 0 {
			//fmt.Println("Выполенно: ", i*100/len(datum), "%")
		} else {
			break
		}
	}
}
