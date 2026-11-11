// whatsappLink opens a WhatsApp chat with phone (+62...), with text typed in
// for the sender to edit before sending.
export const whatsappLink = (phone: string, text: string) =>
  `https://wa.me/${phone.replace(/\D/g, '')}?text=${encodeURIComponent(text)}`
