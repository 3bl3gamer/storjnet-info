export function sortedNodes(nodes) {
	return nodes.sort((a, b) => a.address.localeCompare(b.address))
}
