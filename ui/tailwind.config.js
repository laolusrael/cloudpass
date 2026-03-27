/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	theme: {
		extend: {
			colors: {
				gray: {
					50: '#F5F5F5',
					100: '#E0E0E0',
					200: '#BDBDBD',
					300: '#9E9E9E',
					400: '#757575',
					500: '#616161',
					600: '#424242',
					700: '#212121',
					800: '#1A1A1A',
					900: '#121212'
				}
			},
			spacing: {
				'4.5': '18px',
				'18': '72px'
			}
		}
	},
	plugins: []
};
