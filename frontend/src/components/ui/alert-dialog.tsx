import * as React from 'react'
import { Dialog, DialogContent, DialogHeader, DialogFooter, DialogTitle, DialogDescription } from './dialog'
import { Button } from './button'
import { cn } from '../../lib/utils'

const AlertDialog = Dialog

const AlertDialogContent = React.forwardRef<
  HTMLDivElement,
  React.HTMLAttributes<HTMLDivElement>
>(({ className, ...props }, ref) => (
  <DialogContent ref={ref} className={cn('max-w-md', className)} {...props} />
))
AlertDialogContent.displayName = 'AlertDialogContent'

const AlertDialogHeader = DialogHeader
const AlertDialogFooter = DialogFooter
const AlertDialogTitle = DialogTitle
const AlertDialogDescription = DialogDescription

const AlertDialogCancel = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement>
>(({ children, ...props }, ref) => (
  <Button ref={ref} variant="outline" {...props}>
    {children}
  </Button>
))
AlertDialogCancel.displayName = 'AlertDialogCancel'

const AlertDialogAction = React.forwardRef<
  HTMLButtonElement,
  React.ButtonHTMLAttributes<HTMLButtonElement>
>(({ children, className, ...props }, ref) => (
  <Button ref={ref} className={className} {...props}>
    {children}
  </Button>
))
AlertDialogAction.displayName = 'AlertDialogAction'

export {
  AlertDialog,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogCancel,
  AlertDialogAction,
}
