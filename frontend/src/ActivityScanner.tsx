import {useEffect,useRef,useState} from 'react';
import type jsQR from 'jsqr';
import {studyError} from './study-format';

function scanFrame(decode:typeof jsQR,canvas:HTMLCanvasElement, image:CanvasImageSource, width:number, height:number) {
    const scale=Math.min(1,800/Math.max(width,height));
    canvas.width=Math.max(1,Math.round(width*scale));
    canvas.height=Math.max(1,Math.round(height*scale));
    const context=canvas.getContext('2d',{willReadFrequently:true});
    if(!context)return null;
    context.drawImage(image,0,0,canvas.width,canvas.height);
    const pixels=context.getImageData(0,0,canvas.width,canvas.height);
    return decode(pixels.data,pixels.width,pixels.height,{inversionAttempts:'attemptBoth'})?.data??null;
}

export function ActivityScanner({onCode,disabled}: {onCode:(value:string)=>void;disabled:boolean}) {
    const video=useRef<HTMLVideoElement>(null);
    const photo=useRef<HTMLInputElement>(null);
    const sink=useRef(onCode);
    const [camera,setCamera]=useState(false);
    const [error,setError]=useState('');
    useEffect(()=>{sink.current=onCode},[onCode]);

    useEffect(()=>{
        if(!camera||disabled)return;
        let closed=false;
        let stream:MediaStream|undefined;
        let frame=0;
        let last=0;
        let decoder:typeof jsQR;
        const canvas=document.createElement('canvas');
        function scan(now:number){
            if(closed)return;
            const source=video.current;
            if(source?.videoWidth&&now-last>200){
                last=now;
                const code=scanFrame(decoder,canvas,source,source.videoWidth,source.videoHeight);
                if(code){setCamera(false);sink.current(code);return}
            }
            frame=requestAnimationFrame(scan);
        }
        async function open(){
            try{
                if(!navigator.mediaDevices?.getUserMedia)throw new Error('Камера недоступна. Можно загрузить фото QR-кода.');
                const module=await import('jsqr');
                if(closed)return;
                decoder=module.default;
                stream=await navigator.mediaDevices.getUserMedia({video:{facingMode:{ideal:'environment'}},audio:false});
                if(closed||!video.current){stream.getTracks().forEach(track=>track.stop());return}
                video.current.srcObject=stream;
                await video.current.play();
                if(!closed)frame=requestAnimationFrame(scan);
            }catch(err){
                if(!closed){setError(err instanceof Error&&err.name==='NotAllowedError'?'Разреши доступ к камере или загрузи фото QR-кода.':studyError(err));setCamera(false)}
            }
        }
        void open();
        return()=>{closed=true;cancelAnimationFrame(frame);stream?.getTracks().forEach(track=>track.stop());};
    },[camera,disabled]);

    async function scanFile(file:File){
        setCamera(false);setError('');
        if(file.size>12*1024*1024){setError('Выбери изображение размером до 12 МБ');return}
        try{
            const {default:decoder}=await import('jsqr');
            const bitmap=await createImageBitmap(file);
            try{
                const code=scanFrame(decoder,document.createElement('canvas'),bitmap,bitmap.width,bitmap.height);
                if(!code){setError('На изображении не найден QR-код');return}
                sink.current(code);
            }finally{bitmap.close()}
        }catch(err){setError(studyError(err))}
    }

    return <div className="role-list">
        <div style={{display:'flex',gap:8,flexWrap:'wrap'}}>
            <button className="role-primary" type="button" disabled={disabled} onClick={()=>{setError('');setCamera(value=>!value)}}>
                {camera?'Закрыть камеру':'Сканировать камерой'}
            </button>
            <button className="role-primary" type="button" disabled={disabled} onClick={()=>photo.current?.click()}>Загрузить фото QR-кода</button>
            <input ref={photo} hidden type="file" accept="image/*" disabled={disabled} onChange={event=>{
                const file=event.currentTarget.files?.[0];event.currentTarget.value='';if(file)void scanFile(file);
            }}/>
        </div>
        {camera&&<video ref={video} muted playsInline style={{width:'100%',maxWidth:460,borderRadius:12}}/>}
        {error&&<p className="role-sub" role="alert">{error}</p>}
    </div>;
}
