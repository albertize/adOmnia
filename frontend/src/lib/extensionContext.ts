export type ExtensionContextValues = Record<string, string | number | boolean | null | undefined>

export function evaluateWhen(expression: string | undefined, context: ExtensionContextValues): boolean {
  const source = expression?.trim()
  if (!source) return true
  if (source.length > 1000 || /[\r\n\0]/.test(source)) return false
  return splitOutsideQuotes(source, '||').some((orPart) =>
    splitOutsideQuotes(orPart, '&&').every((term) => evaluateTerm(term.trim(), context)),
  )
}

function evaluateTerm(term: string, context: ExtensionContextValues): boolean {
  if (!term) return true
  if (term.startsWith('!') && !term.startsWith('!=')) return !evaluateTerm(term.slice(1).trim(), context)
  for (const operator of ['!=', '=='] as const) {
    const index = indexOutsideQuotes(term, operator)
    if (index >= 0) {
      const left = context[term.slice(0, index).trim()]
      const right = parseLiteral(term.slice(index + operator.length).trim(), context)
      return operator === '==' ? left === right : left !== right
    }
  }
  return Boolean(context[term])
}

function parseLiteral(value: string, context: ExtensionContextValues): string | number | boolean | null | undefined {
  if ((value.startsWith("'") && value.endsWith("'")) || (value.startsWith('"') && value.endsWith('"'))) return value.slice(1, -1)
  if (value === 'true') return true
  if (value === 'false') return false
  if (value === 'null') return null
  if (value === 'undefined') return undefined
  if (/^-?\d+(?:\.\d+)?$/.test(value)) return Number(value)
  return Object.prototype.hasOwnProperty.call(context, value) ? context[value] : value
}

function splitOutsideQuotes(value: string, separator: string): string[] {
  const parts: string[] = []
  let start = 0
  let quote = ''
  for (let index = 0; index < value.length; index++) {
    const char = value[index]
    if ((char === '"' || char === "'") && value[index - 1] !== '\\') quote = quote === char ? '' : quote || char
    if (!quote && value.slice(index, index + separator.length) === separator) {
      parts.push(value.slice(start, index))
      start = index + separator.length
      index += separator.length - 1
    }
  }
  parts.push(value.slice(start))
  return parts
}

function indexOutsideQuotes(value: string, operator: string): number {
  let quote = ''
  for (let index = 0; index <= value.length - operator.length; index++) {
    const char = value[index]
    if ((char === '"' || char === "'") && value[index - 1] !== '\\') quote = quote === char ? '' : quote || char
    if (!quote && value.slice(index, index + operator.length) === operator) return index
  }
  return -1
}
