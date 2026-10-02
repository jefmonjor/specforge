import { render, screen } from '@testing-library/react'
import { App } from './App'

test('renderiza el titulo del modulo', () => {
  render(<App />)
  expect(screen.getByRole('heading', { level: 1 })).toBeTruthy()
})
