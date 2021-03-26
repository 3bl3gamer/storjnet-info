window.addEventListener('error', e => {
	// looks like plugin error, ignoring it
	if (e.message === 'Script error.' && e.filename === '') return
	// usually, e.message already prefixed with "Uncaught Error:"
	const message = `${e.message} in ${e.filename}:${e.lineno}:${e.colno}`
	sendError(message, e.error)
})

window.addEventListener('unhandledrejection', e => {
	// in case of Promise.reject("string") error will have no message/stack; passing that "reason" as plain text
	const message =
		'Unhandled Rejection: ' + (e.reason && e.reason.message ? e.reason.message : e.reason + '')
	sendError(message, e.reason)
})

/**
 * @param {string} message
 * @param {Error} [error]
 */
export function sendError(message, error) {
	const stack = error && error.stack
	const body = JSON.stringify({ message, stack, url: location.href })
	const headers = { 'Content-Type': 'application/json' }
	fetch('/api/client_errors', { method: 'POST', headers, credentials: 'same-origin', body })
}

export function onError(error) {
	// eslint-disable-next-line no-console
	console.error(error)
	sendError(error + '', error)
}
