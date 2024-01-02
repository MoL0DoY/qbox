package net

// Структура запроса к теплосчётчику
type Request struct {
	Bytes              []byte                     // байты, которые будут посланы в порт теплосчётчика
	ControlFunction    func(response []byte) bool // Функция проверки полученного результата от теплосчётчика
	Attempts           uint8                      // количество попыток перепосылки байтов в порт теплосчётчика в случае ошибки при чтении данных
	Reconnect          bool                       // требуется ли производить переподключение соединения при ошибки EOF
	SecondsReadTimeout uint8                      // таймаут при чтении данных с теплосчётчика
}

// Задаёт настройки по умолчанию для структуры запроса
func PrepareRequest(bytes []byte) Request {
	controlFunction := func(response []byte) bool {
		return true
	}
	return Request{
		Bytes:              bytes,
		ControlFunction:    controlFunction,
		Attempts:           2,
		SecondsReadTimeout: 3,
		Reconnect:          true}
}
