import ws from 'k6/ws';

export const getMsgQueueRequest = ({ value }) => {
    return generateMessage({command: "queue", value });
}
export const getMsgLeaveQueue = () => {
    return generateMessage({ command: "leave_queue" });
}

export const getMsgReadyRoom = ({ value }) => {
    return generateMessage({ command: "ready_queue", value });
}
export const getMsgLeaveRoom = () => {
    return generateMessage({ command: "leave_room" });
}

// TODO: This need to be dynamic.
export const getMsgMovePiece = ({ player_id, piece_id, from, to, is_capture, is_kinged }) => {
    return generateMessage(
        { 
            command: "move_piece", 
            value: {
                player_id,
                piece_id,
                from,
                to,
                is_capture,
                is_kinged
            }
        }
    );
}

const generateMessage = ({ command, value }) => {
    let msg = JSON.stringify(
    {
        command,
        value
    }) 
    
    console.log(msg)
    return msg
}
