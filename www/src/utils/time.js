export const DAY_DURATION = 24 * 3600 * 1000

/** @param {Date|number} date */
export function startOfMonth(date) {
	let newDate = new Date(date)
	newDate.setUTCHours(0, 0, 0, 0)
	newDate.setUTCDate(1)
	return newDate
}
/** @param {Date|number} date */
export function endOfMonth(date) {
	date = startOfMonth(date)
	date.setUTCMonth(date.getUTCMonth() + 1)
	date.setUTCDate(date.getUTCDate() - 1)
	return date
}

/** @param {Date} date */
export function toISODateString(date) {
	return date.toISOString().substr(0, 10)
}

/** @param {{startDate:Date, endDate:Date}} date */
export function toISODateStringInterval({ startDate, endDate }) {
	return { startDateStr: toISODateString(startDate), endDateStr: toISODateString(endDate) }
}

/**
 * @param {string|null} str
 * @param {boolean} [isEnd]
 */
function parseHashIntervalDate(str, isEnd) {
	if (!str) return null
	let m = str.trim().match(/^(\d{4})-(\d\d?)(?:-(\d\d?))?$/)
	if (m === null) return null
	let [, year, month, date] = m

	// this way Date will be at midnight in UTC
	let res = new Date(year + '-' + month.padStart(2, '0') + '-' + (date || '1').padStart(2, '0'))
	if (res.toString() === 'Invalid Date') return null

	if (isEnd && date === undefined) {
		res = endOfMonth(res)
	}
	return res
}
/**
 * @param {Date} date
 * @param {boolean} [isEnd]
 */
function formatHashIntervalDate(date, isEnd) {
	let canTrimDate =
		(!isEnd && date.getTime() === startOfMonth(date).getTime()) ||
		(isEnd && date.getTime() === endOfMonth(date).getTime())
	let str = date.getFullYear() + '-' + (date.getUTCMonth() + 1)
	if (canTrimDate) return str
	return str + '-' + date.getUTCDate()
}

/** @returns {[Date, Date]} */
export function getDefaultHashInterval() {
	let now = new Date()
	return [startOfMonth(now), endOfMonth(now)]
}

export function intervalIsDefault() {
	let [defStart, defEnd] = getDefaultHashInterval()
	let [curStart, curEnd] = getHashInterval()
	return defStart.getTime() == curStart.getTime() && defEnd.getTime() == curEnd.getTime()
}

export function intervalIsMonth() {
	let [start, end] = getHashInterval()
	return +start === +startOfMonth(start) && +end === +endOfMonth(end)
}

/** @returns {[Date, Date]} */
export function getHashInterval() {
	let hash = location.hash.substr(1)
	let params = new URLSearchParams(hash)
	let start = parseHashIntervalDate(params.get('start'))
	let end = parseHashIntervalDate(params.get('end'), true)

	if (start !== null && end !== null && start.getTime() < end.getTime()) {
		return [start, end]
	} else {
		return getDefaultHashInterval()
	}
}

/**
 * @param {Date} startDate
 * @param {Date} endDate
 */
export function makeUpdatedHashInterval(startDate, endDate) {
	let hash = location.hash.substr(1)
	let params = new URLSearchParams(hash)
	params.set('start', formatHashIntervalDate(startDate))
	params.set('end', formatHashIntervalDate(endDate, true))
	return '#' + params.toString()
}

/**
 * @param {(startDate:Date, endDate:Date) => void} onChange
 */
export function watchHashInterval(onChange) {
	function listener() {
		let [startDate, endDate] = getHashInterval()
		onChange(startDate, endDate)
	}
	function off() {
		removeEventListener('hashchange', listener)
	}
	addEventListener('hashchange', listener)

	let [startDate, endDate] = getHashInterval()
	return { startDate, endDate, off }
}
