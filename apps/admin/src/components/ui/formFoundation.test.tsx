import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'
import { Checkbox } from './Checkbox'
import { Input } from './Input'
import { Select } from './Select'
import { Textarea } from './Textarea'

describe('canonical form foundation', () => {
  describe('Input', () => {
    it('associates its label with the control', () => {
      render(<Input label="Email" />)
      expect(screen.getByLabelText('Email')).toBeInstanceOf(HTMLInputElement)
    })

    it('exposes placeholder and accepts typed value', async () => {
      const user = userEvent.setup()
      render(<Input label="Name" placeholder="Type here" />)
      const input = screen.getByLabelText('Name')
      expect(input).toHaveAttribute('placeholder', 'Type here')
      await user.type(input, 'Ada')
      expect(input).toHaveValue('Ada')
    })

    it('marks invalid and associates the error message', () => {
      render(<Input label="Email" error="Required" />)
      const input = screen.getByLabelText('Email')
      expect(input).toHaveAttribute('aria-invalid', 'true')
      const describedBy = input.getAttribute('aria-describedby')
      expect(describedBy).toBeTruthy()
      expect(document.getElementById(describedBy!)).toHaveTextContent('Required')
    })

    it('renders the canonical field surface and focus tokens (normal + compact)', () => {
      const { rerender } = render(<Input label="A" />)
      expect(screen.getByLabelText('A').className).toContain('bg-background')
      expect(screen.getByLabelText('A').className).toContain('focus:ring-ring')
      rerender(<Input label="A" size="compact" />)
      expect(screen.getByLabelText('A').className).toContain('rounded-md')
    })

    it('does not fire onChange when disabled', async () => {
      const user = userEvent.setup()
      const onChange = vi.fn()
      render(<Input label="A" value="" onChange={onChange} disabled />)
      await user.type(screen.getByLabelText('A'), 'x')
      expect(onChange).not.toHaveBeenCalled()
    })
  })

  describe('Select', () => {
    it('associates its label and forwards changes', async () => {
      const user = userEvent.setup()
      render(
        <Select label="Status" defaultValue="a">
          <option value="a">A</option>
          <option value="b">B</option>
        </Select>
      )
      const select = screen.getByLabelText('Status')
      expect(select).toBeInstanceOf(HTMLSelectElement)
      await user.selectOptions(select, 'b')
      expect(select).toHaveValue('b')
    })
  })

  describe('Textarea', () => {
    it('associates its label, error and help', () => {
      render(<Textarea label="Note" error="Too short" />)
      const area = screen.getByLabelText('Note')
      expect(area).toBeInstanceOf(HTMLTextAreaElement)
      expect(area).toHaveAttribute('aria-invalid', 'true')
    })
  })

  describe('Checkbox', () => {
    it('associates its label and toggles checked state', async () => {
      const user = userEvent.setup()
      render(<Checkbox label="Enabled" />)
      const box = screen.getByLabelText('Enabled') as HTMLInputElement
      expect(box).toBeInstanceOf(HTMLInputElement)
      expect(box.type).toBe('checkbox')
      expect(box).not.toBeChecked()
      await user.click(box)
      expect(box).toBeChecked()
    })

    it('uses the canonical primary accent and focus ring', () => {
      render(<Checkbox label="Enabled" />)
      const box = screen.getByLabelText('Enabled')
      expect(box.className).toContain('accent-primary')
      expect(box.className).toContain('focus:ring-ring')
    })
  })
})
