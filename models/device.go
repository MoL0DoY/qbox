package models

import (
	"time"
)

type UnitQEnum byte // Единицы измерения тепловой энергии
const (
	MWh  UnitQEnum = 0x00 // Мегаватты
	Gcal UnitQEnum = 0x01 // ГигаКалории
	GJ   UnitQEnum = 0x02 // ГигаДжоули
	KWh  UnitQEnum = 0x03 // Киловатты
)

/**
Структура данных теплосчётчика.
*/
type DataDevice struct {
	Serial         string         // Серийный заводской номер теплосчётчика
	UnitQ          UnitQEnum      // Единицы измерения тепловой энергии
	TimeRequest    time.Time      // Время запроса
	Time           time.Time      // Время на приборе
	TimeOn         uint32         // Время работы при включенном питании, в секундах
	TimeRunCommon  uint32         // Время работы в нормальном режиме(без ошибок), общее по всем системам, в секундах
	Systems        []SystemDevice // Системы теплосчётчика, нумерация с 0 (в реальности обычно с 1)
	CoefficientGJ  float64        // переводной коэффициент ГДж в ГКал. См. dataDevice::getCoefficientGJ
	CoefficientMWh float64        // переводной коэффициент МВт в ГКал. См. dataDevice::getCoefficientMWh
	CoefficientKWh float64        // переводной коэффициент КВт в ГКал. См. dataDevice::getCoefficientKWh
}

/**
Структура данных для одной системы теплосчётчика.
Примечание: Пока тут заведены только те параметры, которые являются коммерческими.
*/
type SystemDevice struct {
	TimeRunSys uint32 // Время работы без ошибок для системы

	/*
		Тепловые энергии:
		Расчёт тепловой энергии 1, энергии 2, энергии 3 производится в единицах, которых считает счётчик.
		DataDevice::UnitQEnum - должен быть заполнен именно этими единицами измерения.
		Q1 выступает как энергия на подающем теплопроводе, Q2 -энергия на обратном теплопроводе.
		В некоторых конфигурациях теплосчётчиков имеется Q результирующее , это значение следует помещать в SigmaQ.
		Примечание: В некоторых конфигурациях теплосчётчиков заведен расчёт дополнительной энергии, не относящийся к результату
		по подающему, обратному теплопроводу или суммарному значению. Это значение следует помещать в Q3.
	*/
	SigmaQ float64 //ΣQ Результирующее значение тепловой энергии. В некоторых системах это разность Q1,Q2, в некоторых - сумма
	Q1     float64
	Q2     float64
	Q3     float64

	/*
		V1 - Объём на подающем трубопроводе, V2- Объём на обратном трубопроводе.
		Измеряются в метрах кубических (м3)
	*/
	V1 float64
	V2 float64

	/*
		M1 - Масса на подающем трубопроводе, M2- Масса на обратном трубопроводе.
		Измеряются в тоннах (т)
	*/
	M1 float64
	M2 float64

	/*
		GM1, GV1 - расходы на подающем трубопроводе. GM2, GV2 - на обратном трубопроводе.
		Некоторые теплосчётчики фиксируют расход в метрах кубических в час (м3/ч), а другие - в тоннах в час (т/ч)
		Поэтому различают Массовый расход GM - (т/ч) и Объёмный расход GV - (м3/ч).
	*/
	GM1 float32
	GM2 float32
	GV1 float32
	GV2 float32

	/*
		Температуры на подающем (Т1) и обратном(Т2) трубопроводе. Измерения в грудусах цельсия.
		Температуру по трубопроводу холодной воды следует помещать в Т3. В некоторых теплосчётчиках она программируемая, в
		некоторых измеряемая. У нас чаще всего она 10 градусов Цельсия.
	*/
	T1 float32 // Температура 1, в градусах Цельсия
	T2 float32 // Температура 2, в градусах Цельсия
	T3 float32 // Температура 3, в градусах Цельсия

	/*
		Давления  на подающем (P1) и обратном(P2) трубопроводе.
		Теплосчётчики измеряют давление в различных единицах измерения, следует приводить к Мега Паскалям (МПа).
		В некоторых конфигурациях систем заведено дополнительное давление, значение его следует помещать в P3.
		Иногда в роли P3 выступает давление по трубопроводу холодной воды.
	*/
	P1 float32 // Давление 1, в МПа
	P2 float32 // Давление 2, в МПа
	P3 float32 // Давление 3, в МПа

	Status bool // Статус системы, активна или нет. Если нет, то не будет отображаться в результах опроса
}

/**
Добавляет в структуру данных новую систему DataDevice::Systems, согласно переданному номеру(number), если этой системы
ещё не заведено. Номер - это индекс массива, начинается с 0.
Т.е: Если передан номер 2, то будут созданы системы с номером 0 и 1, 2 если это необходимо.
*/
func (dataDevice *DataDevice) AddNewSystem(indexArray int) {
	if len(dataDevice.Systems) == 0 {
		dataDevice.Systems = make([]SystemDevice, indexArray+1, indexArray+1)
	} else if len(dataDevice.Systems)-1 < indexArray {
		newSystems := make([]SystemDevice, (indexArray+1)-(len(dataDevice.Systems)-1))
		dataDevice.Systems = append(dataDevice.Systems, newSystems...)
	}
}

// Изменение единиц измерения энергии
func (dataDevice *DataDevice) ChangeUnitQ(u UnitQEnum) {
	if dataDevice.UnitQ == u {
		return // нужные единицы измерения уже выставлены
	}
	switch u {
	case Gcal:
		dataDevice.toGigaCalories()
		return
	case GJ:
		dataDevice.toGigaJoule()
		return
	case MWh:
		dataDevice.toMegaWatts()
		return
	case KWh:
		dataDevice.toKiloWatts()
		return
	}
}

// Переводные коэффициенты единицы измерения энергии в ГКлал.
// Примечание: Для конечного теплосчётчика эти коэффициенты могут отличаться от заданных,
// поэтому их следует переопределить в драйвере для этого теплосчётчика.
func (dataDevice *DataDevice) getCoefficient() float64 {
	switch dataDevice.UnitQ {
	case GJ:
		return dataDevice.getCoefficientGJ()
	case MWh:
		return dataDevice.getCoefficientMWh()
	case KWh:
		return dataDevice.getCoefficientKWh()
	}

	return 1.0
}

func (dataDevice *DataDevice) toGigaCalories() {
	k := dataDevice.getCoefficient()
	for i := range dataDevice.Systems {
		dataDevice.Systems[i].SigmaQ *= k
		dataDevice.Systems[i].Q1 *= k
		dataDevice.Systems[i].Q2 *= k
		dataDevice.Systems[i].Q3 *= k
	}
	dataDevice.UnitQ = Gcal
}

func (dataDevice *DataDevice) toGigaJoule() {
	dataDevice.toGigaCalories()
	k := dataDevice.getCoefficientGJ()
	for i := range dataDevice.Systems {
		dataDevice.Systems[i].SigmaQ /= k
		dataDevice.Systems[i].Q1 /= k
		dataDevice.Systems[i].Q2 /= k
		dataDevice.Systems[i].Q3 /= k
	}
	dataDevice.UnitQ = GJ
}

func (dataDevice *DataDevice) toMegaWatts() {
	dataDevice.toGigaCalories()
	k := dataDevice.getCoefficientMWh()
	for i := range dataDevice.Systems {
		dataDevice.Systems[i].SigmaQ /= k
		dataDevice.Systems[i].Q1 /= k
		dataDevice.Systems[i].Q2 /= k
		dataDevice.Systems[i].Q3 /= k
	}
	dataDevice.UnitQ = MWh
}

func (dataDevice *DataDevice) toKiloWatts() {
	dataDevice.toGigaCalories()
	k := dataDevice.getCoefficientKWh()
	for i := range dataDevice.Systems {
		dataDevice.Systems[i].SigmaQ /= k
		dataDevice.Systems[i].Q1 /= k
		dataDevice.Systems[i].Q2 /= k
		dataDevice.Systems[i].Q3 /= k
	}
	dataDevice.UnitQ = KWh
}

func (dataDevice *DataDevice) getCoefficientGJ() float64 {
	if dataDevice.CoefficientGJ > 0 {
		return dataDevice.CoefficientGJ
	}
	return 0.239 // по ТКП 411-2012
}

func (dataDevice *DataDevice) getCoefficientMWh() float64 {
	if dataDevice.CoefficientMWh > 0 {
		return dataDevice.CoefficientMWh
	}
	return 0.86 // по ТКП 411-2012
}

func (dataDevice *DataDevice) getCoefficientKWh() float64 {
	if dataDevice.CoefficientKWh > 0 {
		return dataDevice.CoefficientKWh
	}
	return 0.00086 // по ТКП 411-2012
}
