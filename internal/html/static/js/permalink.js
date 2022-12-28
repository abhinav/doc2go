window.addEventListener('load', () => {
	document.querySelectorAll("h2, h3, h4, h5, h6").forEach((el) => {
		if (!el.id) {
			return
		}
		el.innerHTML += ' <a class="permalink" href="#'+el.id+'">&para;</a>'
	})
})
