export default function UserProfileField({
  label,
  value,
  mono = false,
  highlight = false,
}: {
  label: string
  value: string
  mono?: boolean
  highlight?: boolean
}) {
  return (
    <div>
      <p className="text-xs font-medium text-gray-400 mb-0.5">{label}</p>
      <p
        className={`text-sm break-all ${mono ? 'font-mono text-xs' : ''} ${highlight ? 'text-green-600 font-medium' : 'text-gray-800'}`}
      >
        {value}
      </p>
    </div>
  )
}
