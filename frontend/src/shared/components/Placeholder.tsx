export function Placeholder({ name }: { name: string }) {
  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold">{name}</h1>
      <p className="text-gray-500">Coming soon.</p>
    </div>
  )
}
