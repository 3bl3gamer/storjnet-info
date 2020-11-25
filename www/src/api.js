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

export async function apiReq(method, path, params) {
	params = params || {}
	params.method = method

	// подефолту шлём куки
	if (!params.credentials) params.credentials = 'include'

	// если это GET-зпрос, добавляем params.data как query
	if ('data' in params && (!params.method || params.method == 'GET')) {
		let args = []
		for (let key in params.data) {
			let value = params.data[key]
			if (value !== null) args.push(encodeKeyValue(key, value))
		}
		if (args.length > 0) path += '?' + args.join('&')
		delete params.data
	}

	// если это не-GET-запрос, отправляем дату как ЖСОН в боди
	if ('data' in params && params.method != 'GET') {
		params.body = JSON.stringify(params.data)
		delete params.data
	}

	const res = await fetch(path, params)
	if (res.headers.get('Content-Type') === 'application/json') {
		const data = await res.json()
		if (!data.ok) throw new APIError(method, path, res)
		return data.result
	}
	return res
}
