export function mentionAt(text, cursor) {
  const before = text.slice(0, cursor)
  const match = /(?:^|\s)@([^@\s]*)$/.exec(before)
  return match ? { start: cursor - match[1].length - 1, end: cursor, query: match[1] } : null
}
export function insertMention(text, range, member) {
  const token = `@${member.display_name} `
  return { text: text.slice(0, range.start) + token + text.slice(range.end), cursor: range.start + token.length }
}
export function sentMentions(text, selected) {
  return selected.filter(member => text.includes(`@${member.name}`)).map(({id,name}) => ({id,name}))
}
