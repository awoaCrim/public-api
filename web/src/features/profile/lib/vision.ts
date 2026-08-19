/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
/**
 * The default prompt is configuration data sent to the vision model, so it is
 * intentionally kept in English rather than translated as interface copy.
 */
export const DEFAULT_VISION_PROMPT = `Describe the provided image as accurately and comprehensively as possible, based strictly on visible evidence.

Preserve all information that may be useful to another AI, including subjects, objects, appearance, actions, colors, materials, spatial relationships, layout, foreground/background, lighting, perspective, composition, UI elements, symbols, diagrams, charts, and visible text.

Transcribe readable text exactly. Mark unreadable text as [illegible].

Do not guess, invent, or infer unsupported identities, locations, relationships, intentions, events, brands, or hidden details. Explicitly mark uncertain observations as uncertain.

Prioritize factual accuracy, spatial relationships, and information preservation over elegant prose or brevity.`

export function normalizeVisionPrompt(prompt: string | null | undefined) {
  return prompt?.trim() ? prompt : DEFAULT_VISION_PROMPT
}
