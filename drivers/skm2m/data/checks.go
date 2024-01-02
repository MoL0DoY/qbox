package data

import "qBox/services/log"

type Checks struct {
	Logger *log.LoggerService
}

/**
Проверка контрольной суммы для СКМ-2
Представляет собой сумму значений из bytes, урезанную до одного байта
*/
func (checks *Checks) CalculateCheckSum(bytes []byte) byte {
	var sum byte = 0
	for i := 0; i < len(bytes); i++ {
		sum = sum + bytes[i]
	}
	return sum & 0xFF
}

// Чтение Single Character, согласно протоколу СКМ-2
func (checks *Checks) CheckSimpleFrame(response []byte) bool {
	if len(response) == 0 {
		checks.Logger.Info("Получен пустой ответ.")
		return false
	}

	if len(response) > 0 && response[0] == 0xE5 {
		return true
	} else {
		checks.Logger.Info("Получен некорректный ответ. Проверка SimpleFrame не пройдена.")
		return false
	}
}

func (checks *Checks) CheckLongFrame(response []byte) bool {
	if len(response) < 19 {
		// Структура ответа на запрос текущих данных занимает от 19 байт и больше
		checks.Logger.Info("Получен некорректный ответ. Ответ содержит меньше 19 байт.")
		return false
	}

	if response[0] != response[3] || response[0] != 0x68 || response[1] != response[2] {
		checks.Logger.Info(
			"Получен некорректный ответ. Заголовок(68h L L 68h) ответа не верный - %X",
			response[0:3])
		return false
	}

	L := response[1] // задаём L из заголовка

	if len(response) < int(L)+4+1+1 { //  Заголовок(68h L L 68h) + L + CheckSum + Stop(16h)
		checks.Logger.Info("Получен некорректный ответ. Некорректная длинна ответа, согласно заголовку (68h L L 68h).")
		return false
	}

	checkSum := response[len(response)-2]
	calculatedCheckSum := checks.CalculateCheckSum(response[4 : len(response)-2])
	if calculatedCheckSum != checkSum {
		checks.Logger.Info("Получен некорректный ответ. Контрольная сумма не совпадает.")
		checks.Logger.Debug("Ожидалась контрольная сумма- %X", calculatedCheckSum)
		checks.Logger.Debug("Получена контрольная сумма- %X", checkSum)
		return false
	}

	return true
}
