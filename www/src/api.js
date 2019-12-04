function encodeKeyValue(key, value) {
	return encodeURIComponent(key) + '=' + encodeURIComponent(value)
}

function handleRawResult(res) {
	if (res.headers.get('Content-Type') === 'application/json') {
		return res.json().then(processJsonResult)
	}
	return res
}

class APIError extends Error {
	constructor(res) {
		let msg = res.error
		if (res.description) msg += ': ' + res.description
		super(msg)
		this.error = res.error
		this.description = res.description
	}
}

function processJsonResult(res) {
	if (!res.ok) {
		throw new APIError(res)
	}
	return res.result
}

export function apiReq(method, path, params) {
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

	return fetch(path, params).then(handleRawResult)
}
