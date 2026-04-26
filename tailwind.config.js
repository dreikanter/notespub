/** @type {import('tailwindcss').Config} */
module.exports = {
  theme: {
    extend: {
      typography: {
        DEFAULT: {
          css: {
            'code::before': {
              content: '""'
            },
            'code::after': {
              content: '""'
            },
            'code': {
              fontSize: '0.9em',
            },
            'pre': {
              fontSize: '0.9em',
              lineHeight: '1.5',
              borderRadius: '0.125rem',
              paddingTop: '0.75rem',
              paddingBottom: '0.75rem',
              paddingLeft: '1rem',
              paddingRight: '1rem',
            },
            'h2 code': {
              fontSize: 'inherit',
            },
            'h3 code': {
              fontSize: 'inherit',
            },
            'a': {
              color: '#075985', // sky-800
            },
            'a:hover': {
              color: '#0284c7', // sky-600
            },
          }
        }
      },
    },
  },
}
