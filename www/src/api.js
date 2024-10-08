import { lang } from './i18n'

function encodeKeyValue(key, value) {
	return encodeURIComponent(key) + '=' + encodeURIComponent(value)
}

class APIError extends Error {
	constructor(method, url, res) {
		let msg = `${method}:${url} -> ${res.error}`
		if (res.description) msg += ': ' + res.description
		super(msg)
		this.error = res.error
		this.description = res.description
	}
}

/**
 * @param {'GET'|'POST'|'DELETE'} method
 * @param {string} path
 * @param {(Parameters<typeof fetch>[1] & {data?:Record<string,unknown>})?} [params]
 */
export async function apiReq(method, path, params) {
	params = params || {}
	params.method = method

	// подефолту шлём куки
	if (!params.credentials) params.credentials = 'include'

	// если это GET-зпрос, добавляем params.data как query
	if ('data' in params && (!params.method || params.method === 'GET')) {
		let args = []
		for (let key in params.data) {
			let value = params.data[key]
			if (value !== null) args.push(encodeKeyValue(key, value))
		}
		if (args.length > 0) path += '?' + args.join('&')
		delete params.data
	}

	// если это не-GET-запрос, отправляем дату как ЖСОН в боди
	if ('data' in params && params.method !== 'GET') {
		params.body = JSON.stringify(params.data)
		delete params.data
	}

	const res = await fetch(path, params)
	if (res.headers.get('Content-Type') === 'application/json') {
		const data = await res.json()
		if (!data.ok) throw new APIError(method, path, data)
		return data.result
	}
	return res
}

/**
 * @typedef {{
 *   ips: Record<string, {
 *     sanction: null | NodeIPSanction,
 *     fullInfo?: {
 *       country: {name:string, geoNameID:number, isoCode:string},
 *       city: {name:string, geoNameID:number},
 *       registeredCountry: {name:string, geoNameID:number, isoCode:string},
 *       subdivisions:{name:string, geoNameID:number, isoCode:string}[],
 *     },
 *   }>,
 * }} IPsSanctionsResponse
 */

/**
 * @typedef {{reason: string, detail: string}} NodeIPSanction
 */

/**
 * @param {string[]} ips
 * @param {boolean} fullInfo
 * @param {AbortController|null} abortController
 * @returns {Promise<IPsSanctionsResponse>}
 */
export function apiReqIPsSanctions(ips, fullInfo, abortController) {
	return apiReq('POST', '/api/ips_sanctions', {
		data: { ips, lang, fullInfo },
		signal: abortController?.signal,
	})
}
