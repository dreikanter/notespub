/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./templates/**/*.html",
    "./templates/**/*.xml",
  ],
  safelist: [
    "bg-slate-100",
    "bg-slate-500",
    "block",
    "border",
    "border-slate-200",
    "border-slate-600",
    "font-normal",
    "no-underline",
    "rounded-md",
    "text-slate-100",
    "text-slate-400",
    "text-slate-800",
    "text-base",
    "leading-normal",
    "overflow-x-auto",
    "rounded-sm",
    {
      "pattern": /m(l|r|x|y)-.+/,
    },
    {
      "pattern": /p(l|r|x|y)-.+/,
    },
  ],
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
  plugins: [
    require('@tailwindcss/typography')
  ],
}
