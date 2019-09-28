!(function() {
	let wrap = document.getElementById('mapWrap')
	let map = new Map(wrap, Map.proj.Mercator)
	map.updateLocation(0, 34, Math.log2(map.canvas.getBoundingClientRect().width))
	if (map.top_left_y_shift < 0) map.move(0, map.top_left_y_shift)

	var tileContainer = new MapTileContainer(
		256,
		function(x, y, z) {
			//return `https://c.basemaps.cartocdn.com/rastertiles/dark_all/${z}/${x}/${y}@1x.png`
			return `https://c.basemaps.cartocdn.com/rastertiles/light_all/${z}/${x}/${y}@1x.png`
		},
		Map.proj.Mercator,
	)
	map.register(new MapTileLayer(tileContainer))
	// map.register(new MapURLLayer(map))
	map.register(new MapControlLayer(map))
	let pointsLayer = new PointsLayer()
	map.register(pointsLayer)
	//map.register(new MapLocationLayer(map));

	map.register(new MapControlHintLayer(wrap.dataset.hintControlText, wrap.dataset.hintTwoFingersText))

	function onResize() {
		map.resize()
	}
	addEventListener('resize', onResize)
	onResize()

	fetch('/node_locations.bin').then(r => r.arrayBuffer()).then(buf => {
		let vals = new Uint16Array(buf)
		let points = new Array(vals.length/2)
		for (let i=0; i<points.length; i++)
			points[i] = [vals[i*2]/65536*360-180, vals[i*2+1]/65536*180-90]
		pointsLayer.setLocations(points)
	})
})()
