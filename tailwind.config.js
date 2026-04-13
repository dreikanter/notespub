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
              fontSize: '1em',
            },
            'pre': {
              fontSize: '1em',
            },
            'h2 code': {
              fontSize: 'inherit',
            },
            'h3 code': {
              fontSize: 'inherit',
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
