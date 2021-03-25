/**
 * @template T
 * @param {Promise<T>|unknown} value
 * @returns {value is Promise<T>}
 */
export function isPromise(value) {
	return typeof value === 'object' && value !== null && 'then' in value
}
