if (!Object.fromEntries) {
	Object.fromEntries = entries => {
		const obj = {}
		for (const entry of entries) obj[entry[0]] = entry[1]
		return obj
	}
}
